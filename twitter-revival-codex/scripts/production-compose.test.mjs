import { test } from "node:test";
import assert from "node:assert/strict";
import { execFileSync } from "node:child_process";
import path from "node:path";
import { randomBytes } from "node:crypto";
import {
  mkdir,
  mkdtemp,
  readFile,
  writeFile,
  unlink,
  rmdir,
} from "node:fs/promises";
import { projectRoot } from "./production-config.mjs";

const docker =
  process.platform === "win32"
    ? path.join(
        process.env.LOCALAPPDATA,
        "Programs/DockerDesktop/resources/bin/docker.exe",
      )
    : "docker";
const environment = {
  ...process.env,
  DOMAIN: "beta.fixture-radura.it",
  ACME_EMAIL: "operator@fixture-radura.it",
  RELEASE_TAG: "deployment-check",
  PUBLIC_BETA_ENABLED: "false",
  POSTGRES_PASSWORD: "a".repeat(64),
  S3_ACCESS_KEY: "fixture-access-key",
  S3_SECRET_KEY: "b".repeat(64),
  METRICS_TOKEN: "c".repeat(64),
  SMTP_ADDR: "smtp.fixture-radura.it:587",
  SMTP_FROM: "Radura <no-reply@beta.fixture-radura.it>",
  SMTP_USERNAME: "fixture-user",
  SMTP_PASSWORD: "fixture-password",
};
if (process.platform === "win32") {
  const pathKey =
    Object.keys(environment).find((key) => key.toLowerCase() === "path") ??
    "Path";
  environment[pathKey] =
    path.dirname(docker) + path.delimiter + (environment[pathKey] ?? "");
}
function run(args) {
  return execFileSync(docker, args, {
    cwd: projectRoot,
    env: environment,
    stdio: "pipe",
    timeout: 30000,
  }).toString();
}
test("standalone server topology exposes only HTTPS/HTTP and loopback monitoring", () => {
  const config = JSON.parse(
    run([
      "compose",
      "--env-file",
      ".env.production.example",
      "-f",
      "compose.production.yaml",
      "config",
      "--format",
      "json",
    ]),
  );
  for (const name of ["db", "storage", "migrate", "api", "web"])
    assert.ok(!config.services[name].ports, `${name} must have no host ports`);
  assert.deepEqual(
    config.services.gateway.ports
      .map((p) => Number(p.published))
      .sort((a, b) => a - b),
    [80, 443],
  );
  assert.equal(config.services.prometheus.ports[0].host_ip, "127.0.0.1");
  assert.equal(config.networks.backend.internal, true);
  assert.ok(!config.services.mailpit);
  assert.equal(
    config.services.api.environment.APP_ORIGIN,
    "https://beta.fixture-radura.it",
  );
  assert.equal(config.services.api.environment.COOKIE_SECURE, "true");
  assert.equal(config.services.api.environment.BETA_INVITE_ONLY, "true");
  assert.equal(config.services.api.environment.SMTP_STARTTLS, "true");
  assert.equal(
    config.services.api.environment.SMTP_ALLOW_LOCAL_PLAINTEXT,
    "false",
  );
  assert.equal(config.services.web.build.args.NEXT_PUBLIC_DEV_MAILBOX_URL, "");
  assert.equal(
    config.services.gateway.environment.PUBLIC_BETA_ENABLED,
    "false",
  );
  assert.equal(
    config.volumes.postgres_data.name,
    "radura-production_postgres_data",
  );
});
test("Caddy production configuration adapts in both modes without contacting an ACME service", () => {
  for (const enabled of ["false", "true"]) {
    const config = JSON.parse(
      run([
        "run",
        "--rm",
        "--network",
        "none",
        "-e",
        "DOMAIN=beta.fixture-radura.it",
        "-e",
        "ACME_EMAIL=operator@fixture-radura.it",
        "-e",
        `PUBLIC_BETA_ENABLED=${enabled}`,
        "-v",
        `${path.join(projectRoot, "infra/Caddyfile.production")}:/etc/caddy/Caddyfile:ro`,
        "caddy:2.11.7-alpine",
        "caddy",
        "adapt",
        "--config",
        "/etc/caddy/Caddyfile",
      ]),
    );
    const text = JSON.stringify(config);
    assert.ok(text.includes(`${enabled} != true`));
    assert.ok(text.includes("web:3000"));
    assert.ok(!text.includes('"module":"internal"'));
  }
});

test("domain-only Compose starts just the closed gateway, with no host ports or application data", async () => {
  const project = `radura-domain-check-${randomBytes(4).toString("hex")}`;
  assert.match(project, /^radura-domain-check-[a-f0-9]{8}$/);
  const base = path.join(projectRoot, ".local");
  await mkdir(base, { recursive: true });
  const folder = await mkdtemp(path.join(base, "production-domain-test-"));
  const override = path.join(folder, "gateway.yaml");
  await writeFile(
    override,
    "services:\n  gateway:\n    ports: !override []\n    environment:\n      DOMAIN: http://localhost:8181\n      PUBLIC_BETA_ENABLED: 'false'\nnetworks:\n  frontend:\n    internal: true\n",
  );
  const compose = [
    "compose",
    "--project-name",
    project,
    "--env-file",
    ".env.production.example",
    "-f",
    "compose.production.yaml",
    "-f",
    override,
  ];
  let created = false;
  try {
    const config = JSON.parse(run([...compose, "config", "--format", "json"]));
    assert.equal(config.services.gateway.ports?.length ?? 0, 0);
    assert.equal(config.networks.frontend.internal, true);
    for (const volume of Object.values(config.volumes))
      assert.ok(!volume.external && volume.name.startsWith(project + "_"));
    created = true;
    run([
      ...compose,
      "up",
      "-d",
      "--no-deps",
      "--no-build",
      "--wait",
      "--wait-timeout",
      "20",
      "gateway",
    ]);
    const containers = run([...compose, "ps", "--format", "json"])
      .trim()
      .split(/\r?\n/)
      .filter(Boolean)
      .map((line) => JSON.parse(line));
    assert.deepEqual(
      containers.map((c) => c.Service),
      ["gateway"],
    );
    assert.equal(containers[0].Health, "healthy");
  } finally {
    if (created) run([...compose, "down", "--volumes"]);
    await unlink(override);
    await rmdir(folder);
  }
});
test("closed gateway serves maintenance, hides metrics and retains container health offline", () => {
  const script = `
caddy run --config /etc/caddy/Caddyfile >/tmp/check.log 2>&1 &
checker_pid=$!
trap 'kill "$checker_pid" 2>/dev/null || true' EXIT
ready=0
for i in 1 2 3 4 5 6 7 8 9 10; do
 if wget -q -O /dev/null http://127.0.0.1:8080/healthz; then ready=1; break; fi
 sleep 0.2
done
if [ "$ready" != 1 ]; then cat /tmp/check.log; exit 1; fi
wget -S -O /dev/null http://localhost:8181/ 2>&1 || true
wget -S -O /dev/null http://localhost:8181/metrics 2>&1 || true
`;
  const output = run([
    "run",
    "--rm",
    "--network",
    "none",
    "--entrypoint",
    "sh",
    "-e",
    "DOMAIN=http://localhost:8181",
    "-e",
    "ACME_EMAIL=operator@fixture-radura.it",
    "-e",
    "PUBLIC_BETA_ENABLED=false",
    "-v",
    `${path.join(projectRoot, "infra/Caddyfile.production")}:/etc/caddy/Caddyfile:ro`,
    "caddy:2.11.7-alpine",
    "-c",
    script,
  ]);
  assert.match(output, /HTTP\/1\.1 503/);
  assert.match(output, /HTTP\/1\.1 404/);
});
test("production shell entrypoint has valid Bash syntax", async (t) => {
  const base = path.join(projectRoot, ".local");
  await mkdir(base, { recursive: true });
  const root = await mkdtemp(path.join(base, "production-bash-test-"));
  t.after(async () => {
    await unlink(path.join(root, "start-production.sh"));
    await rmdir(root);
  });
  // Git on Windows may use CRLF; the release must normalize shell scripts.
  await writeFile(
    path.join(root, "start-production.sh"),
    (
      await readFile(
        path.join(projectRoot, "scripts/start-production.sh"),
        "utf8",
      )
    ).replaceAll("\r\n", "\n"),
  );
  run([
    "run",
    "--rm",
    "--network",
    "none",
    "--entrypoint",
    "bash",
    "-v",
    `${root}:/checks:ro`,
    "postgres:18-alpine",
    "-n",
    "/checks/start-production.sh",
  ]);
});
