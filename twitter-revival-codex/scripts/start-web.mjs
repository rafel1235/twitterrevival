import { access, cp } from "node:fs/promises";
import { spawn } from "node:child_process";
import path from "node:path";
import { fileURLToPath } from "node:url";

const web = fileURLToPath(new URL("../apps/web/", import.meta.url));
const runtime = path.join(web, ".next", "standalone", "apps", "web");
try {
  await access(path.join(runtime, "server.js"));
} catch {
  console.error("Build mancante: esegui npm run build dalla root.");
  process.exit(1);
}
await cp(
  path.join(web, ".next", "static"),
  path.join(runtime, ".next", "static"),
  { recursive: true },
);
const child = spawn(process.execPath, [path.join(runtime, "server.js")], {
  stdio: "inherit",
  env: {
    ...process.env,
    HOSTNAME: process.env.WEB_HOST ?? "127.0.0.1",
    PORT: process.env.PORT ?? "3000",
  },
});
for (const signal of ["SIGINT", "SIGTERM"])
  process.on(signal, () => child.kill(signal));
child.on("error", () => {
  console.error("Avvio frontend non riuscito.");
  process.exit(1);
});
child.on("exit", (code) => process.exit(code ?? 1));
