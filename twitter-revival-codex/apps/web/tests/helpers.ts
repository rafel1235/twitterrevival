import { test, type APIRequestContext, type Page } from "@playwright/test";
import { execFileSync } from "node:child_process";
import path from "node:path";
const testCohort = `e2e-${Date.now().toString(36)}-${process.pid}`;
export function localCLI(args: string[]) {
  const cli = process.env.E2E_API_CLI;
  if (cli)
    return execFileSync(path.resolve(cli), args, {
      stdio: "pipe",
      timeout: 20000,
    }).toString();
  const docker =
    process.platform === "win32"
      ? path.join(
          process.env.LOCALAPPDATA!,
          "Programs/DockerDesktop/resources/bin/docker.exe",
        )
      : "docker";
  return execFileSync(
    docker,
    ["compose", "exec", "-T", "api", "/api", ...args],
    { stdio: "pipe", timeout: 20000 },
  ).toString();
}
export function testInvite(email: string) {
  return JSON.parse(
    localCLI([
      "-invite-email",
      email,
      "-invite-cohort",
      testCohort,
      "-test-cohort",
    ]),
  ).token as string;
}

async function respectRetryAfter(value: string | undefined) {
  const seconds = Number(value);
  if (!Number.isFinite(seconds) || seconds < 1 || seconds > 60)
    throw new Error("429 senza un Retry-After valido (1–60 secondi).");
  await test.step(`Rispetto il Retry-After del server: ${seconds}s`, async () => {
    await new Promise((resolve) => setTimeout(resolve, seconds * 1000 + 100));
  });
}
// Keep the real limiter enabled. Browser contexts share Next's peer quota.
export async function registerAccount(
  client: APIRequestContext,
  origin: string,
  data: unknown,
) {
  const beta = await client.get("/api/v1/beta");
  const needsInvite = beta.ok() && (await beta.json()).data.invite_required;
  const input = data as Record<string, unknown>;
  const payload =
    needsInvite && !input.invite_token
      ? { ...input, invite_token: testInvite(String(input.email)) }
      : input;
  for (let attempt = 0; attempt < 3; attempt++) {
    const response = await client.post("/api/v1/auth/register", {
      headers: { Origin: origin, "X-CSRF-Protection": "1" },
      data: payload,
    });
    if (response.status() !== 429) return response;
    await respectRetryAfter(response.headers()["retry-after"]);
  }
  throw new Error("Quota auth ancora esaurita dopo il Retry-After.");
}
export async function submitAuth(
  page: Page,
  action: "register" | "login" | "logout",
  click: () => Promise<void>,
) {
  if (action === "register") {
    const field = page.getByLabel("Codice invito", { exact: true });
    if ((await field.count()) && !(await field.inputValue()))
      await field.fill(
        testInvite(
          await page.getByLabel("Email", { exact: true }).inputValue(),
        ),
      );
  }
  for (let attempt = 0; attempt < 3; attempt++) {
    const result = page.waitForResponse(
      (response) =>
        new URL(response.url()).pathname === `/api/v1/auth/${action}` &&
        response.request().method() === "POST",
    );
    await click();
    const response = await result;
    if (response.status() !== 429) return response;
    await respectRetryAfter(response.headers()["retry-after"]);
  }
  throw new Error("Quota auth ancora esaurita dopo il Retry-After.");
}
