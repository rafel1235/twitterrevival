import { expect, test } from "@playwright/test";
import { localCLI, registerAccount, submitAuth } from "./helpers";

test("@auth beta privata: invito dal pannello, frammento rimosso, registrazione e revoca", async ({
  page,
  context,
  browser,
  baseURL,
}, testInfo) => {
  test.skip(process.env.E2E_AUTH !== "1", "Richiede API e PostgreSQL reali.");
  test.setTimeout(240000);
  const origin = baseURL!;
  const headers = { Origin: origin, "X-CSRF-Protection": "1" };
  const tag = Date.now().toString(36);
  const handle = `bmod_${tag}`;
  const email = `pilot_${tag}@example.test`;
  const secondEmail = `revoke_${tag}@example.test`;
  const password = "password beta locale sicura 🔐";
  const registered = await registerAccount(context.request, origin, {
    handle,
    email: `${handle}@example.test`,
    display_name: "Moderatore beta",
    password,
  });
  expect(registered.status()).toBe(201);
  const role = (value: string) =>
    localCLI([
      "-role-user",
      handle,
      "-role",
      value,
      "-reason",
      "Verifica E2E locale del pannello beta con account sintetico.",
    ]);
  role("moderator");
  const pilot = await browser.newContext({
    baseURL: origin,
    ignoreHTTPSErrors: origin === "https://localhost:3443",
  });
  let created = false;
  let paused = false;
  let initialCapacity = 50;
  try {
    await page.goto("/moderation/beta");
    await expect(
      page.getByRole("heading", { name: "Beta privata" }),
    ).toBeVisible();
    await expect(
      page.getByText("Non ancora misurabile", { exact: true }),
    ).toBeVisible();
    const initial = (
      await (await context.request.get("/api/v1/moderation/beta")).json()
    ).data.beta;
    initialCapacity = initial.capacity;
    expect(initial.enrollment_open).toBe(true);
    await page.getByLabel("Email del destinatario").fill(email);
    await page
      .getByRole("button", { name: "Crea invito", exact: true })
      .click();
    const field = page.getByLabel("Collegamento privato");
    await expect(field).toBeVisible();
    const link = await field.inputValue();
    expect(new URL(link).search).toBe("");
    const token = new URL(link).hash.slice("#invite=".length);
    expect(token.length).toBe(43);
    await page.getByRole("button", { name: "Nascondi collegamento" }).click();
    await expect(field).toHaveCount(0);
    const projection = await (
      await context.request.get("/api/v1/moderation/beta/invites")
    ).text();
    expect(projection).not.toContain(token);
    const pilotPage = await pilot.newPage();
    await pilotPage.goto(link);
    await expect(
      pilotPage.getByLabel("Codice invito", { exact: true }),
    ).toHaveValue(token);
    await expect(pilotPage).toHaveURL(`${origin}/register`);
    await pilotPage
      .getByLabel("Come ti chiami?")
      .fill("Persona pilota di prova");
    await pilotPage.getByLabel("Handle", { exact: true }).fill(`pilot_${tag}`);
    await pilotPage.getByLabel("Email", { exact: true }).fill(email);
    await pilotPage.getByLabel("Password", { exact: true }).fill(password);
    expect(
      (
        await submitAuth(pilotPage, "register", () =>
          pilotPage
            .getByRole("button", { name: "Crea account", exact: true })
            .click(),
        )
      ).status(),
    ).toBe(201);
    created = true;
    await expect(pilotPage).toHaveURL("/");
    expect((await pilot.request.get("/api/v1/moderation/beta")).status()).toBe(
      403,
    );
    await pilotPage.goto("/moderation/beta");
    await expect(
      pilotPage.getByText("Accesso riservato ai moderatori."),
    ).toBeVisible();
    const stats = (
      await (await context.request.get("/api/v1/moderation/beta")).json()
    ).data.beta;
    expect(stats.members).toBe(initial.members + 1);
    await page.getByLabel("Email del destinatario").fill(secondEmail);
    await page
      .getByRole("button", { name: "Crea invito", exact: true })
      .click();
    await expect(field).toBeVisible();
    const row = page.getByRole("article").filter({ hasText: secondEmail });
    await row.getByRole("button", { name: "Revoca invito" }).click();
    await expect(row).toContainText("Revocato");
    await page.getByRole("button", { name: "Nascondi collegamento" }).click();
    paused = true;
    await page
      .getByRole("button", { name: "Metti in pausa gli ingressi" })
      .click();
    await expect(
      page.getByRole("button", { name: "Riapri gli ingressi" }),
    ).toBeVisible();
    expect(
      (await (await pilot.request.get("/api/v1/beta")).json()).data
        .enrollment_open,
    ).toBe(false);
    await page.getByRole("button", { name: "Riapri gli ingressi" }).click();
    await expect(
      page.getByRole("button", { name: "Metti in pausa gli ingressi" }),
    ).toBeVisible();
    paused = false;
    expect(
      await page.evaluate(
        () => document.documentElement.scrollWidth <= innerWidth,
      ),
    ).toBe(true);
    await page.screenshot({
      path: testInfo.outputPath(`beta-${testInfo.project.name}.png`),
      fullPage: true,
    });
  } finally {
    if (paused)
      await context.request.patch("/api/v1/moderation/beta", {
        headers,
        data: { capacity: initialCapacity, enrollment_open: true },
      });
    const list = await context.request.get("/api/v1/moderation/beta/invites");
    if (list.ok()) {
      for (const invite of (await list.json()).data.items) {
        if (
          [email, secondEmail].includes(invite.email) &&
          invite.status === "available"
        )
          await context.request.delete(
            `/api/v1/moderation/beta/invites/${invite.id}`,
            { headers, data: {} },
          );
      }
    }
    if (created)
      expect(
        (
          await pilot.request.delete("/api/v1/account", {
            headers,
            data: { password, confirmation: "ELIMINA" },
          })
        ).status(),
      ).toBe(204);
    role("member");
    expect(
      (
        await context.request.delete("/api/v1/account", {
          headers,
          data: { password, confirmation: "ELIMINA" },
        })
      ).status(),
    ).toBe(204);
    await pilot.close();
  }
});
