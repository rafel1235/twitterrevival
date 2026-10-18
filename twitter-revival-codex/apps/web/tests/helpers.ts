import { test, type APIRequestContext, type Page } from "@playwright/test";

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
  for (let attempt = 0; attempt < 3; attempt++) {
    const response = await client.post("/api/v1/auth/register", {
      headers: { Origin: origin, "X-CSRF-Protection": "1" },
      data,
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
