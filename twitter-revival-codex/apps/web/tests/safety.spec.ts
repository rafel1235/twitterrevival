import { expect, test, type APIRequestContext } from "@playwright/test";
import { execFileSync } from "node:child_process";
import { readFile } from "node:fs/promises";
import path from "node:path";
import { registerAccount } from "./helpers";

const password = "password sprint cinque sicura 🔐";
const headers = (origin: string) => ({
  Origin: origin,
  "X-CSRF-Protection": "1",
});
async function register(
  client: APIRequestContext,
  origin: string,
  tag: string,
) {
  const handle = `${tag}_${Date.now().toString(36)}`;
  const response = await registerAccount(client, origin, {
    handle,
    email: `${handle}@example.test`,
    display_name: `Radura ${tag}`,
    password,
  });
  expect(response.status()).toBe(201);
  return (await response.json()).data.user;
}
function setRole(handle: string, role: "moderator" | "member") {
  const args = [
    "-role-user",
    handle,
    "-role",
    role,
    "-reason",
    "Account creato dal test E2E locale per verificare la moderazione.",
  ];
  if (process.env.E2E_API_CLI)
    execFileSync(path.resolve(process.env.E2E_API_CLI), args, {
      stdio: "pipe",
      timeout: 20000,
    });
  else {
    const docker =
      process.platform === "win32"
        ? path.join(
            process.env.LOCALAPPDATA!,
            "Programs/DockerDesktop/resources/bin/docker.exe",
          )
        : "docker";
    execFileSync(docker, ["compose", "exec", "-T", "api", "/api", ...args], {
      stdio: "pipe",
      timeout: 20000,
    });
  }
}
test("privacy accessibile e impostazioni protette senza sessione", async ({
  page,
}) => {
  await page.goto("/privacy");
  await expect(
    page.getByRole("heading", { name: "Dati e sicurezza" }),
  ).toBeVisible();
  for (const route of [
    "/settings/account",
    "/settings/reports",
    "/moderation",
  ]) {
    await page.goto(route);
    await expect(
      page.getByRole("heading", { name: "Entra nella community" }),
    ).toBeVisible();
  }
});
test("@auth segnalazione, decisione moderatore, ripristino e audit reali", async ({
  page,
  context,
  browser,
  baseURL,
}, testInfo) => {
  test.skip(process.env.E2E_AUTH !== "1", "Richiede API e PostgreSQL reali.");
  test.setTimeout(180000);
  const origin = baseURL!;
  await register(context.request, origin, "report");
  const author = await browser.newContext({
    baseURL: origin,
    ignoreHTTPSErrors: origin === "https://localhost:3443",
  });
  const mod = await browser.newContext({
    baseURL: origin,
    ignoreHTTPSErrors: origin === "https://localhost:3443",
  });
  let moderatorHandle = "";
  try {
    await register(author.request, origin, "author");
    const moderator = await register(mod.request, origin, "review");
    moderatorHandle = moderator.handle;
    setRole(moderator.handle, "moderator");
    const created = await author.request.post("/api/v1/posts", {
      headers: headers(origin),
      data: {
        body: "Una conversazione da verificare con gli strumenti della community 🌿",
      },
    });
    expect(created.status()).toBe(201);
    const post = (await created.json()).data.post;
    await page.goto(`/posts/${post.id}`);
    await page.getByText("Segnala post", { exact: true }).click();
    const form = page.getByRole("form", { name: "Invia segnalazione" });
    await form
      .getByLabel("Dettagli")
      .fill("Richiedo una verifica manuale di questo contenuto di prova.");
    const submitted = page.waitForResponse(
      (r) =>
        new URL(r.url()).pathname === "/api/v1/reports" &&
        r.request().method() === "POST",
    );
    await form.getByRole("button", { name: "Invia segnalazione" }).click();
    const reportId = (await (await submitted).json()).data.id;
    await expect(
      page.getByRole("status").filter({ hasText: "Segnalazione ricevuta" }),
    ).toBeVisible();
    await page.goto("/settings/reports");
    await expect(
      page.getByText("Da esaminare", { exact: true }).last(),
    ).toBeVisible();
    const forbidden = await context.request.get("/api/v1/moderation/reports");
    expect(forbidden.status()).toBe(403);
    const review = await mod.newPage();
    await review.goto("/moderation");
    // Locate the new report by ID so pre-existing local reports remain untouched.
    const card = review.getByRole("article", {
      name: `Segnalazione ${reportId}`,
      exact: true,
    });
    await expect(card).toBeVisible();
    await card.getByLabel("Azione", { exact: true }).selectOption("hide_post");
    await card
      .getByLabel("Motivazione della decisione")
      .fill("Contenuto nascosto per verifica nel test locale della community.");
    const saved = review.waitForResponse(
      (r) => r.url().includes("/decisions") && r.request().method() === "POST",
    );
    await card.getByRole("button", { name: "Conferma decisione" }).click();
    expect((await saved).status()).toBe(204);
    await page.goto(`/posts/${post.id}`);
    await expect(
      page.getByText("Post non disponibile.", { exact: true }),
    ).toBeVisible();
    await page.goto("/settings/reports");
    await expect(
      page.getByText(
        "Contenuto nascosto per verifica nel test locale della community.",
      ),
    ).toBeVisible();
    await review.getByLabel("Stato", { exact: true }).selectOption("resolved");
    const resolved = review.getByRole("article", {
      name: `Segnalazione ${reportId}`,
      exact: true,
    });
    await expect(resolved).toBeVisible();
    await review.screenshot({
      path: testInfo.outputPath(`moderation-${testInfo.project.name}.png`),
      fullPage: true,
    });
    await resolved
      .getByLabel("Azione", { exact: true })
      .selectOption("restore_post");
    await resolved
      .getByLabel("Motivazione della decisione")
      .fill("Contenuto ripristinato dopo la verifica manuale del test locale.");
    const restored = review.waitForResponse(
      (r) => r.url().includes("/decisions") && r.request().method() === "POST",
    );
    await resolved.getByRole("button", { name: "Conferma decisione" }).click();
    expect((await restored).status()).toBe(204);
    await page.goto(`/posts/${post.id}`);
    await expect(page.getByText(post.body, { exact: true })).toBeVisible();
    await review.getByRole("button", { name: "Apri audit" }).click();
    await expect(
      review
        .getByRole("heading", { name: "Ripristina post", exact: true })
        .first(),
    ).toBeVisible();
  } finally {
    if (moderatorHandle) setRole(moderatorHandle, "member");
    await author.close();
    await mod.close();
  }
});
test("@auth esportazione ZIP con immagine e cancellazione definitiva", async ({
  page,
  context,
  baseURL,
}, testInfo) => {
  test.skip(
    process.env.E2E_AUTH !== "1",
    "Richiede API, PostgreSQL e S3 reali.",
  );
  test.setTimeout(180000);
  const origin = baseURL!;
  await register(context.request, origin, "privacy");
  const image = await readFile(
    path.join(process.cwd(), "apps/web/tests/fixtures/radura-landscape.png"),
  );
  const uploaded = await context.request.post("/api/v1/media", {
    headers: headers(origin),
    multipart: {
      alt_text: "Paesaggio del test di esportazione",
      file: { name: "landscape.png", mimeType: "image/png", buffer: image },
    },
  });
  expect(uploaded.status()).toBe(201);
  const media = (await uploaded.json()).data.media;
  const created = await context.request.post("/api/v1/posts", {
    headers: headers(origin),
    data: {
      body: "I miei dati esportabili e poi eliminati 🌿",
      media_ids: [media.id],
    },
  });
  expect(created.status()).toBe(201);
  await page.goto("/settings/profile");
  await page.getByRole("link", { name: "I tuoi dati", exact: true }).click();
  const currentPassword = page.getByLabel("Password attuale", { exact: true });
  await currentPassword.fill("password lunga ma errata");
  await page.getByRole("button", { name: "Scarica i tuoi dati" }).click();
  await expect(
    page.getByRole("alert").filter({ hasText: "password attuale" }),
  ).toBeVisible();
  await currentPassword.fill(password);
  const downloadEvent = page.waitForEvent("download");
  await page.getByRole("button", { name: "Scarica i tuoi dati" }).click();
  const download = await downloadEvent;
  expect(download.suggestedFilename()).toBe("radura-export.zip");
  const archivePath = testInfo.outputPath("private-account-export.zip");
  await download.saveAs(archivePath);
  const archive = await readFile(archivePath);
  expect(archive.subarray(0, 2).toString()).toBe("PK");
  expect(archive.length).toBeGreaterThan(image.length);
  await expect(
    page.getByRole("status").filter({ hasText: "Archivio pronto" }),
  ).toBeVisible();
  await page.screenshot({
    path: testInfo.outputPath(`account-${testInfo.project.name}.png`),
    fullPage: true,
  });
  await currentPassword.fill(password);
  await page.getByLabel("Scrivi ELIMINA").fill("ELIMINA");
  const deleted = page.waitForResponse(
    (r) =>
      new URL(r.url()).pathname === "/api/v1/account" &&
      r.request().method() === "DELETE",
  );
  await page
    .getByRole("button", { name: "Elimina definitivamente il mio account" })
    .click();
  expect((await deleted).status()).toBe(204);
  await expect(
    page.getByRole("heading", { name: "Account eliminato", exact: true }),
  ).toBeVisible();
  expect((await context.request.get("/api/v1/auth/me")).status()).toBe(401);
  await page.reload();
  await expect(
    page.getByRole("heading", { name: "Entra nella community" }),
  ).toBeVisible();
});
