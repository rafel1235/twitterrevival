import { expect, test, type APIRequestContext } from "@playwright/test";
import { registerAccount, submitAuth } from "./helpers";

const password = "identity password sicura 🔐";
async function register(
  client: APIRequestContext,
  origin: string,
  tag: string,
) {
  const handle = `${tag}_${Date.now().toString(36)}`;
  const email = `${handle}@example.test`;
  const response = await registerAccount(client, origin, {
    handle,
    email,
    display_name: handle,
    password,
  });
  expect(response.status()).toBe(201);
  const payload = await response.json();
  return { ...payload.data.user, email };
}
const headers = (origin: string) => ({
  Origin: origin,
  "X-CSRF-Protection": "1",
});

test("profili e impostazioni richiedono una sessione", async ({ page }) => {
  await page.goto("/settings/profile");
  await expect(
    page.getByRole("heading", { name: "Entra nella community" }),
  ).toBeVisible();
  await expect(
    page.getByRole("form", { name: "Modifica profilo" }),
  ).toHaveCount(0);
});

test("@auth profilo persistente, follow reciproci e blocco reale", async ({
  page,
  context,
  browser,
  baseURL,
}, testInfo) => {
  test.skip(process.env.E2E_AUTH !== "1", "Richiede API e PostgreSQL reali.");
  test.setTimeout(150000);
  const origin = baseURL!;
  const a = await register(context.request, origin, "graph_a");
  const other = await browser.newContext({ baseURL: origin });
  try {
    const b = await register(other.request, origin, "graph_b");
    await page.goto("/settings/profile");
    const form = page.getByRole("form", { name: "Modifica profilo" });
    await expect(form).toBeVisible();
    await form
      .getByLabel("Nome visualizzato", { exact: true })
      .fill(`Lina ${testInfo.project.name}`);
    await form.getByLabel("Bio", { exact: true }).fill("🌿".repeat(161));
    await form.getByRole("button", { name: "Salva profilo" }).click();
    await expect(
      form.getByText("Massimo 160 caratteri.", { exact: true }),
    ).toBeVisible();
    const bio = "Idee, caffè e conversazioni. 🌿";
    await form.getByLabel("Bio", { exact: true }).fill(bio);
    await form.getByLabel("Località", { exact: true }).fill("Roma");
    await form
      .getByLabel("Sito web", { exact: true })
      .fill("https://example.test/idee");
    await form.getByRole("radio", { name: "Lavanda", exact: true }).check();
    await form.getByRole("button", { name: "Salva profilo" }).click();
    await expect(form.getByRole("status")).toHaveText("Profilo aggiornato.");
    await page.goto(`/u/${a.handle}`);
    await expect(
      page.getByRole("heading", {
        level: 1,
        name: `Lina ${testInfo.project.name}`,
      }),
    ).toBeVisible();
    await expect(page.getByText(bio, { exact: true })).toBeVisible();
    await page.reload();
    await expect(page.getByText(bio, { exact: true })).toBeVisible();
    expect(
      (
        await context.request.patch("/api/v1/users/me", {
          headers: headers(origin),
          data: { display_name: "W".repeat(50), bio: "🌿".repeat(160) },
        })
      ).status(),
    ).toBe(200);
    await page.reload();
    await expect(
      page.getByRole("heading", { level: 1, name: "W".repeat(50) }),
    ).toBeVisible();
    expect(
      await page.evaluate(
        () => document.documentElement.scrollWidth <= window.innerWidth,
      ),
    ).toBe(true);
    expect(
      (
        await context.request.patch("/api/v1/users/me", {
          headers: headers(origin),
          data: { display_name: `Lina ${testInfo.project.name}`, bio },
        })
      ).status(),
    ).toBe(200);
    await page.goto(`/u/${b.handle}`);
    await page.getByRole("button", { name: "Segui", exact: true }).click();
    await expect(
      page.getByRole("button", { name: "Non seguire più", exact: true }),
    ).toBeVisible();
    await page.reload();
    await expect(
      page.getByRole("button", { name: "Non seguire più", exact: true }),
    ).toBeVisible();
    await page
      .getByRole("button", { name: "Non seguire più", exact: true })
      .click();
    await expect(
      page.getByRole("button", { name: "Segui", exact: true }),
    ).toBeVisible();
    await page.getByRole("button", { name: "Segui", exact: true }).click();
    await expect(
      page.getByRole("button", { name: "Non seguire più", exact: true }),
    ).toBeVisible();
    expect(
      (
        await other.request.post(`/api/v1/users/${a.id}/follow`, {
          headers: headers(origin),
          data: {},
        })
      ).status(),
    ).toBe(204);
    await page.reload();
    await expect(page.getByText("Ti segue", { exact: true })).toBeVisible();
    expect(
      await page.evaluate(
        () => document.documentElement.scrollWidth <= window.innerWidth,
      ),
    ).toBe(true);
    await page.screenshot({
      path: testInfo.outputPath(`profile-${testInfo.project.name}.png`),
      fullPage: true,
    });
    await page.getByRole("button", { name: "Blocca", exact: true }).click();
    await page
      .getByRole("button", { name: "Conferma blocco", exact: true })
      .click();
    await expect(page).toHaveURL("/settings/blocks");
    await expect(
      page.getByRole("button", { name: `Sblocca @${b.handle}`, exact: true }),
    ).toBeVisible();
    expect(
      (await other.request.get(`/api/v1/users/${a.handle}`)).status(),
    ).toBe(404);
    await page
      .getByRole("button", { name: `Sblocca @${b.handle}`, exact: true })
      .click();
    await expect(
      page.getByText("Non hai bloccato nessuno.", { exact: true }),
    ).toBeVisible();
    await page.goto(`/u/${b.handle}`);
    await expect(
      page.getByRole("button", { name: "Segui", exact: true }),
    ).toBeVisible();
    await expect(page.getByText("Ti segue", { exact: true })).toHaveCount(0);
  } finally {
    await other.close();
  }
});

type MailMessage = { ID: string; Subject: string };
async function actionLink(
  client: APIRequestContext,
  email: string,
  subject: string,
  origin: string,
) {
  const inbox = process.env.MAILPIT_URL || "http://localhost:8025";
  let id = "";
  await expect
    .poll(
      async () => {
        const response = await client.get(
          `${inbox}/api/v1/search?query=${encodeURIComponent(`to:${email}`)}`,
        );
        expect(response.ok()).toBe(true);
        const data = await response.json();
        id =
          (data.messages as MailMessage[]).find((message) =>
            message.Subject.includes(subject),
          )?.ID || "";
        return id;
      },
      { timeout: 15000 },
    )
    .not.toBe("");
  const response = await client.get(`${inbox}/api/v1/message/${id}`);
  expect(response.ok()).toBe(true);
  const mail = await response.json();
  const match = String(mail.Text).match(
    /https?:\/\/[^\s]+\?token=[A-Za-z0-9_-]{43}/,
  );
  expect(match).not.toBeNull();
  const link = match![0];
  expect(new URL(link).origin).toBe(new URL(origin).origin);
  return link;
}

test("@auth verifica email e recupero password via SMTP reale", async ({
  page,
  context,
  request,
  browser,
  baseURL,
}, testInfo) => {
  test.skip(
    process.env.E2E_AUTH !== "1" || process.env.E2E_MAILPIT !== "1",
    "Richiede API, PostgreSQL e Mailpit reali; E2E_MAILPIT=1.",
  );
  test.setTimeout(150000);
  const origin = baseURL!;
  const a = await register(context.request, origin, "mail");
  const oldSession = await browser.newContext({ baseURL: origin });
  await oldSession.addCookies(await context.cookies());
  try {
    await page.goto(
      await actionLink(request, a.email, "Conferma la tua email", origin),
    );
    await page
      .getByRole("button", { name: "Conferma email", exact: true })
      .click();
    await expect(
      page.getByRole("status").filter({ hasText: "Email verificata." }),
    ).toBeVisible();
    await page.goto("/settings/profile");
    await expect(
      page.getByText("Email verificata", { exact: true }),
    ).toBeVisible();
    await page.goto("/forgot-password");
    const recovery = page.getByRole("form", { name: "Recupero password" });
    await recovery.getByLabel("Email", { exact: true }).fill(a.email);
    await recovery
      .getByRole("button", { name: "Invia link di recupero" })
      .click();
    await expect(
      page.getByRole("status").filter({ hasText: "Se l'indirizzo" }),
    ).toBeVisible();
    await page.goto(
      await actionLink(request, a.email, "Recupero password", origin),
    );
    const form = page.getByRole("form", { name: "Nuova password" });
    const replacement = "nuova password per il test 🔐";
    await form.getByLabel("Nuova password", { exact: true }).fill(replacement);
    await form
      .getByLabel("Conferma password", { exact: true })
      .fill(replacement);
    await form.getByRole("button", { name: "Aggiorna password" }).click();
    await expect(
      page.getByRole("status").filter({ hasText: "Password aggiornata." }),
    ).toBeVisible();
    expect((await oldSession.request.get("/api/v1/auth/me")).status()).toBe(
      401,
    );
    await page.goto("/login");
    await page.getByLabel("Email", { exact: true }).fill(a.email);
    await page.getByLabel("Password", { exact: true }).fill(replacement);
    await submitAuth(page, "login", () =>
      page.getByRole("button", { name: "Accedi", exact: true }).click(),
    );
    await expect(page).toHaveURL("/");
    await expect(page.getByTestId("session-name")).toHaveText(a.display_name);
    await page.goto("/settings/profile");
    await expect(
      page.getByText("Email verificata", { exact: true }),
    ).toBeVisible();
    await page.screenshot({
      path: testInfo.outputPath(
        `account-security-${testInfo.project.name}.png`,
      ),
      fullPage: true,
    });
  } finally {
    await oldSession.close();
  }
});
