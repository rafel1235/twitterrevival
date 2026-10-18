import { test } from "node:test";
import assert from "node:assert/strict";
import {
  mkdir,
  mkdtemp,
  readFile,
  writeFile,
  unlink,
  rmdir,
  chmod,
} from "node:fs/promises";
import path from "node:path";
import {
  projectRoot,
  parseConfig,
  validateConfig,
  initialize,
  readConfig,
  readDomainConfig,
  prepare,
  publicDomain,
} from "./production-config.mjs";

const valid = () => ({
  DOMAIN: "radura-fixture.it",
  ACME_EMAIL: "operator@radura-fixture.it",
  RELEASE_TAG: "beta-001",
  PUBLIC_BETA_ENABLED: "false",
  POSTGRES_PASSWORD: "a".repeat(64),
  S3_ACCESS_KEY: "radura-fixture-key",
  S3_SECRET_KEY: "b".repeat(64),
  METRICS_TOKEN: "c".repeat(64),
  SMTP_ADDR: "smtp.relay-fixture.it:587",
  SMTP_FROM: "Radura <no-reply@radura-fixture.it>",
  SMTP_USERNAME: "fixture-user",
  SMTP_PASSWORD: "private-fixture-password",
});
const serialize = (config) =>
  Object.entries(config)
    .map(([key, value]) => `${key}=${value}`)
    .join("\n") + "\n";
test("server config accepts a private beta with distinct secrets and required TLS relay", () => {
  assert.deepEqual(validateConfig(valid()), []);
  assert.deepEqual(parseConfig(serialize(valid())), valid());
  assert.equal(
    parseConfig("SMTP_PASSWORD='literal$with#symbols'").SMTP_PASSWORD,
    "literal$with#symbols",
  );
});
test("server config rejects loopback, placeholders, unsafe DSN secrets and implicit SMTP TLS", () => {
  for (const [key, value] of [
    ["DOMAIN", "localhost"],
    ["DOMAIN", "https://radura-fixture.it"],
    ["DOMAIN", "127.0.0.1"],
    ["DOMAIN", "beta.example.invalid"],
    ["POSTGRES_PASSWORD", "unsafe@password"],
    ["S3_SECRET_KEY", "a".repeat(64)],
    ["SMTP_ADDR", "mailpit:1025"],
    ["SMTP_ADDR", "smtp.relay-fixture.it:465"],
    ["SMTP_USERNAME", "SET_ME"],
    ["SMTP_FROM", "Radura <no-reply@someone-else.it>"],
    ["PUBLIC_BETA_ENABLED", "yes"],
    ["RELEASE_TAG", "../bad"],
  ]) {
    const config = { ...valid(), [key]: value };
    const errors = validateConfig(config);
    assert.ok(errors.length, `${key} was accepted`);
    assert.ok(!errors.join("\n").includes("private-fixture-password"));
  }
  for (const domain of [
    "example.com",
    "beta.example.org",
    "radura.local",
    "bad..it",
    "x.it/path",
  ])
    assert.equal(publicDomain(domain), false);
});
test("parser rejects interpolation, commands, duplicate and unknown keys without exposing values", () => {
  for (const input of [
    "SMTP_PASSWORD=$SECRET",
    'SMTP_PASSWORD="private-fixture-password"',
    "DOMAIN=x.it\nDOMAIN=y.it",
    "UNEXPECTED=private-fixture-password",
    "$(private-fixture-password)",
  ])
    assert.throws(
      () => parseConfig(input),
      (error) => !error.message.includes("private-fixture-password"),
    );
});
test("initialization preserves files, generates separate credentials, and prepare writes only the metrics secret", async (t) => {
  const base = path.join(projectRoot, ".local");
  await mkdir(base, { recursive: true });
  const root = await mkdtemp(path.join(base, "production-config-test-"));
  t.after(async () => {
    for (const file of [".env.production", ".local/production/metrics-token"])
      await unlink(path.join(root, file)).catch((error) => {
        if (error.code !== "ENOENT") throw error;
      });
    for (const dir of [".local/production", ".local", ""])
      await rmdir(path.join(root, dir)).catch((error) => {
        if (error.code !== "ENOENT") throw error;
      });
  });
  const generated = await initialize(
    root,
    "radura-fixture.it",
    "operator@radura-fixture.it",
  );
  assert.equal(
    new Set([
      generated.POSTGRES_PASSWORD,
      generated.S3_SECRET_KEY,
      generated.METRICS_TOKEN,
    ]).size,
    3,
  );
  const original = await readFile(path.join(root, ".env.production"), "utf8");
  await assert.rejects(
    () => initialize(root, "radura-fixture.it", "operator@radura-fixture.it"),
    { code: "EEXIST" },
  );
  assert.equal(
    await readFile(path.join(root, ".env.production"), "utf8"),
    original,
  );
  await assert.rejects(() => readConfig(path.join(root, ".env.production")));
  assert.equal(
    (await readDomainConfig(path.join(root, ".env.production")))
      .PUBLIC_BETA_ENABLED,
    "false",
  );
  await writeFile(
    path.join(root, ".env.production"),
    serialize({ ...generated, PUBLIC_BETA_ENABLED: "true" }),
  );
  await assert.rejects(
    () => readDomainConfig(path.join(root, ".env.production")),
    /PUBLIC_BETA_ENABLED=false/,
  );
  const config = {
    ...generated,
    SMTP_ADDR: valid().SMTP_ADDR,
    SMTP_USERNAME: valid().SMTP_USERNAME,
    SMTP_PASSWORD: valid().SMTP_PASSWORD,
  };
  await writeFile(path.join(root, ".env.production"), serialize(config));
  await prepare(root);
  assert.equal(
    await readFile(path.join(root, ".local/production/metrics-token"), "utf8"),
    config.METRICS_TOKEN,
  );
  assert.equal(
    await readFile(path.join(root, ".env.production"), "utf8"),
    serialize(config),
  );
  if (process.platform !== "win32") {
    await chmod(path.join(root, ".env.production"), 0o644);
    await assert.rejects(
      () => readConfig(path.join(root, ".env.production")),
      /chmod 600/,
    );
    await chmod(path.join(root, ".env.production"), 0o600);
  }
});
