import { expect, test } from "@playwright/test";
import https from "node:https";
import { readFile } from "node:fs/promises";
import { registerAccount } from "./helpers";

test("HTTPS locale: catena CA verificata, redirect, cookie Secure e Origin", async ({
  page,
  context,
  request,
  baseURL,
}, testInfo) => {
  test.setTimeout(120000);
  const origin = baseURL!;
  const ca = await readFile(
    process.env.LOCAL_TLS_CA ?? ".local/caddy-root.crt",
  );
  const tls = await new Promise<{
    status: number | undefined;
    authorized: boolean;
    protocol: string | null;
  }>((resolve, reject) => {
    const call = https.get(
      `${origin}/api/v1/beta`,
      { ca, rejectUnauthorized: true },
      (response) => {
        const socket = response.socket as import("node:tls").TLSSocket;
        const value = {
          status: response.statusCode,
          authorized: socket.authorized,
          protocol: socket.getProtocol(),
        };
        response.resume();
        response.on("end", () => resolve(value));
      },
    );
    call.on("error", reject);
  });
  expect(tls.status).toBe(200);
  expect(tls.authorized).toBe(true);
  expect(["TLSv1.2", "TLSv1.3"]).toContain(tls.protocol);
  const redirect = await request.get("http://localhost:3080/register", {
    maxRedirects: 0,
  });
  expect(redirect.status()).toBe(308);
  expect(redirect.headers().location).toBe(`${origin}/register`);
  expect((await request.get("/metrics")).status()).toBe(404);
  const handle = `tls_${Date.now().toString(36)}`;
  const password = "password HTTPS locale sicura 🔐";
  const response = await registerAccount(context.request, origin, {
    handle,
    email: `${handle}@example.test`,
    display_name: "Persona HTTPS",
    password,
  });
  expect(response.status()).toBe(201);
  const cookie = (await context.cookies()).find(
    (c) => c.name === "__Host-radura_session",
  );
  expect(cookie).toMatchObject({
    httpOnly: true,
    secure: true,
    sameSite: "Lax",
    path: "/",
  });
  const headers = { Origin: origin, "X-CSRF-Protection": "1" };
  try {
    await page.goto("/");
    await expect(page.getByTestId("session-name")).toHaveText("Persona HTTPS");
    await page.reload();
    await expect(page.getByTestId("session-name")).toHaveText("Persona HTTPS");
    const post = await context.request.post("/api/v1/posts", {
      headers,
      data: { body: "Post verificato attraverso HTTPS locale 🌿" },
    });
    expect(post.status()).toBe(201);
    const id = (await post.json()).data.post.id;
    await page.goto(`/posts/${id}`);
    await expect(
      page.getByText("Post verificato attraverso HTTPS locale 🌿", {
        exact: true,
      }),
    ).toBeVisible();
    expect(
      (
        await context.request.post("/api/v1/auth/logout", {
          headers: { Origin: "https://evil.test", "X-CSRF-Protection": "1" },
          data: {},
        })
      ).status(),
    ).toBe(403);
    expect((await context.request.get("/api/v1/auth/me")).status()).toBe(200);
    expect(
      await page.evaluate(
        () => document.documentElement.scrollWidth <= innerWidth,
      ),
    ).toBe(true);
    await page.screenshot({
      path: testInfo.outputPath(`https-${testInfo.project.name}.png`),
      fullPage: true,
    });
  } finally {
    expect(
      (
        await context.request.delete("/api/v1/account", {
          headers,
          data: { password, confirmation: "ELIMINA" },
        })
      ).status(),
    ).toBe(204);
  }
  expect((await context.request.get("/api/v1/auth/me")).status()).toBe(401);
});
