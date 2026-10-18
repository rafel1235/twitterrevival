import { spawn } from "node:child_process";
import { randomBytes } from "node:crypto";
import { mkdir, readFile } from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";
import assert from "node:assert/strict";

const root = fileURLToPath(new URL("../", import.meta.url));
const project = `radura-backend-check-${randomBytes(4).toString("hex")}`;
assert.match(project, /^radura-backend-check-[a-f0-9]{8}$/);
const results = path.join(root, ".local", "backend-tests", project);
await mkdir(results, { recursive: true });
for (const directory of ["gocache-linux", "gopath"])
  await mkdir(path.join(root, ".local", directory), { recursive: true });
const docker =
  process.platform === "win32"
    ? path.join(
        process.env.LOCALAPPDATA,
        "Programs/DockerDesktop/resources/bin/docker.exe",
      )
    : "docker";
const env = {
  ...process.env,
  BACKEND_TEST_RESULTS: results,
  PATH:
    process.platform === "win32"
      ? `${path.dirname(docker)}${path.delimiter}${process.env.PATH ?? ""}`
      : process.env.PATH,
};
const args = [
  "compose",
  "--project-name",
  project,
  "--env-file",
  ".env.example",
  "-f",
  "infra/backend-tests.compose.yaml",
];
async function run(command, capture = false) {
  return new Promise((resolve, reject) => {
    const child = spawn(docker, [...args, ...command], {
      cwd: root,
      env,
      stdio: capture ? ["ignore", "pipe", "inherit"] : "inherit",
    });
    let output = "";
    child.stdout?.on("data", (chunk) => {
      output += chunk;
    });
    child.on("error", reject);
    child.on("close", (code) =>
      code === 0
        ? resolve(output)
        : reject(new Error(`Docker test command exited with code ${code}`)),
    );
  });
}

// Reject any accidental connection to application networks, volumes or credentials.
const config = JSON.parse(await run(["config", "--format", "json"], true));
assert.deepEqual(Object.keys(config.services).sort(), [
  "checks",
  "db",
  "storage",
]);
assert.equal(Object.keys(config.volumes ?? {}).length, 0);
for (const service of Object.values(config.services)) {
  assert.equal(service.ports?.length ?? 0, 0);
  assert.equal(service.network_mode, undefined);
}
assert.equal(
  config.services.db.environment.POSTGRES_PASSWORD,
  "isolated-test-password",
);
assert.equal(
  config.services.checks.environment.TEST_DATABASE_URL,
  "postgres://radura_test:isolated-test-password@db:5432/radura_test?sslmode=disable",
);
assert.equal(
  config.services.checks.environment.S3_SECRET_KEY,
  "isolated-test-secret-never-used-for-application-data",
);
assert.deepEqual(config.services.db.volumes ?? [], []);
assert.deepEqual(config.services.storage.volumes ?? [], []);
const mounts = new Map(
  config.services.checks.volumes.map((volume) => [volume.target, volume]),
);
assert.equal(mounts.size, 4);
for (const [target, source] of [
  ["/src", "services/api"],
  ["/cache", ".local/gocache-linux"],
  ["/go", ".local/gopath"],
  ["/results", path.relative(root, results)],
]) {
  const mount = mounts.get(target);
  assert.equal(mount.type, "bind");
  assert.equal(
    path.resolve(mount.source).toLowerCase(),
    path.resolve(root, source).toLowerCase(),
  );
  assert.equal(!!mount.read_only, target === "/src");
}
for (const network of Object.values(config.networks)) {
  assert.equal(network.external ?? false, false);
  assert.ok(network.name.startsWith(`${project}_`));
}
console.log(
  `Backend isolato: ${project}. Nessuna porta host; DB e S3 effimeri. Report: ${results}`,
);
try {
  await run(["run", "--rm", "checks"]);
  const events = (await readFile(path.join(results, "go-test.jsonl"), "utf8"))
    .trim()
    .split("\n")
    .map((line) => JSON.parse(line));
  const tests = events.filter(
    (event) => event.Test && ["pass", "fail", "skip"].includes(event.Action),
  );
  const failures = tests.filter((event) => event.Action !== "pass");
  assert.ok(tests.length > 0);
  assert.deepEqual(failures, []);
  console.log(
    JSON.stringify({
      passed: tests.length,
      topLevel: tests.filter((event) => !event.Test.includes("/")).length,
      subtests: tests.filter((event) => event.Test.includes("/")).length,
      failed: 0,
      skipped: 0,
      gofmt: "passed",
      vet: "passed",
      build: "passed",
    }),
  );
} finally {
  // The random project was validated before startup; only its ephemeral resources are removed.
  await run(["down", "--volumes", "--remove-orphans"]);
}
