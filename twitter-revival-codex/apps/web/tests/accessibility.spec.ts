import AxeBuilder from "@axe-core/playwright";
import { test, expect } from "./ux-fixtures";
import type { Page, TestInfo } from "@playwright/test";

async function accessible(page: Page, info: TestInfo, name: string) {
  const results = await new AxeBuilder({ page })
    .withTags(["wcag2a", "wcag2aa", "wcag21a", "wcag21aa", "wcag22aa"])
    .analyze();
  await info.attach(`axe-${name}`, {
    body: JSON.stringify(results, null, 2),
    contentType: "application/json",
  });
  expect
    .soft(
      results.violations.map(({ id, nodes }) => ({
        id,
        targets: nodes.map((node) => node.target),
        reasons: nodes.map((node) => node.failureSummary),
      })),
    )
    .toEqual([]);
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= innerWidth,
    ),
  ).toBe(true);
}

test("@ux pagine pubbliche: axe e layout responsive", async ({
  page,
}, info) => {
  test.setTimeout(90000);
  for (const path of [
    "/",
    "/login",
    "/register",
    "/forgot-password",
    "/reset-password",
    "/verify-email",
    "/privacy",
    "/pagina-inesistente",
  ]) {
    await page.goto(path);
    await expect(page.getByRole("heading", { level: 1 })).toBeVisible();
    if (path === "/")
      await expect(page.getByRole("note")).toContainText("Feed demo");
    if (path === "/register")
      await expect(
        page.getByLabel("Codice invito", { exact: true }),
      ).toBeVisible();
    await accessible(page, info, path.replaceAll("/", "_") || "home");
  }
});

test("@ux tastiera: salto al contenuto, navigazione con nome e pagina corrente", async ({
  page,
}, info) => {
  await page.goto("/");
  await expect(
    page.getByRole("heading", { name: "Home", exact: true }),
  ).toBeVisible();
  await page.keyboard.press("Tab");
  await expect(
    page.getByRole("link", { name: "Vai al contenuto" }),
  ).toBeFocused();
  await page.keyboard.press("Enter");
  await expect(page.getByRole("main")).toBeFocused();
  await page.keyboard.press("Tab");
  expect(
    await page
      .getByRole("main")
      .evaluate((main) => main.contains(document.activeElement)),
  ).toBe(true);
  const nav = page.getByRole("navigation", {
    name:
      info.project.name === "mobile"
        ? "Navigazione mobile"
        : "Navigazione principale",
  });
  await expect(
    nav.getByRole("link", { name: "Home", exact: true }),
  ).toHaveAttribute("aria-current", "page");
  await expect(
    nav.getByRole("link", { name: "Esplora", exact: true }),
  ).toBeVisible();
  await nav.getByRole("link", { name: "Esplora", exact: true }).click();
  await expect(
    nav.getByRole("link", { name: "Esplora", exact: true }),
  ).toHaveAttribute("aria-current", "page");
});

test("@ux errori di accesso: focus sul campo o sul messaggio e dati conservati", async ({
  page,
}, info) => {
  await page.goto("/login");
  const form = page.getByRole("form", { name: "Accesso" });
  const email = form.getByLabel("Email", { exact: true });
  await email.fill("collaudo@example.test");
  await form
    .getByLabel("Password", { exact: true })
    .fill("password di collaudo lunga");
  await page.route("**/api/v1/auth/login", (route) =>
    route.fulfill({
      status: 422,
      json: {
        error: {
          message: "Controlla i campi.",
          fields: { email: "Controlla questo indirizzo." },
        },
      },
    }),
  );
  await form.getByRole("button", { name: "Accedi", exact: true }).click();
  await expect(email).toBeFocused();
  await expect(email).toHaveAccessibleDescription(
    "Controlla questo indirizzo.",
  );
  await page.unroute("**/api/v1/auth/login");
  await page.route("**/api/v1/auth/login", (route) =>
    route.fulfill({
      status: 401,
      json: { error: { message: "Credenziali non valide." } },
    }),
  );
  await form.getByRole("button", { name: "Accedi", exact: true }).click();
  await expect(form.getByRole("alert")).toBeFocused();
  await expect(email).toHaveValue("collaudo@example.test");
  await accessible(page, info, "errore-accesso");
});

test("@ux reflow a 320 px e testo ingrandito", async ({ page }, info) => {
  await page.setViewportSize({ width: 320, height: 800 });
  await page.goto("/login");
  await expect(page.getByRole("form", { name: "Accesso" })).toBeVisible();
  await accessible(page, info, "320px");
  await page.setViewportSize({
    width: Math.max(768, info.project.use.viewport?.width ?? 1440),
    height: 900,
  });
  await page.evaluate(() => {
    const elements = Array.from(
      document.querySelectorAll<HTMLElement>("body, body *"),
    )
      .filter((element) => element instanceof HTMLElement)
      .map((element) => ({
        element,
        size: parseFloat(getComputedStyle(element).fontSize),
      }));
    for (const { element, size } of elements)
      element.style.fontSize = `${size * 2}px`;
  });
  await expect(page.getByLabel("Email", { exact: true })).toBeVisible();
  await page.getByLabel("Email", { exact: true }).fill("collaudo@example.test");
  await page
    .getByLabel("Password", { exact: true })
    .fill("password di collaudo");
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= innerWidth,
    ),
  ).toBe(true);
  for (const element of await page
    .locator(
      ".brand-name, .welcome-card h2, .desktop-nav .nav-item > span:not(.notification-icon)",
    )
    .all()) {
    if (await element.isVisible())
      expect(
        await element.evaluate(
          (node) => node.scrollWidth <= node.clientWidth + 1,
        ),
        "Il testo ingrandito deve rimanere nel proprio contenitore, senza essere tagliato",
      ).toBe(true);
  }
  await page.screenshot({
    path: info.outputPath("testo-ingrandito.png"),
    fullPage: true,
  });
});

test.describe("@auth @ux percorsi autenticati", () => {
  test.skip(
    process.env.E2E_AUTH !== "1",
    "Richiede la beta locale con API e database reali.",
  );
  test.beforeEach(async ({ context, pilot }) => {
    await context.addCookies(pilot.storage.cookies);
  });

  test("axe su feed, conversazione, profilo, ricerca, notifiche e impostazioni", async ({
    page,
    pilot,
  }, info) => {
    test.setTimeout(120000);
    for (const path of [
      "/",
      `/posts/${pilot.post.id}`,
      `/posts/${pilot.quote.id}`,
      `/u/${pilot.user.handle}`,
      "/people",
      "/notifications",
      "/search?q=collaudo&type=posts",
      "/settings/profile",
      "/settings/account",
      "/settings/blocks",
      "/settings/reports",
    ]) {
      await page.goto(path);
      await expect(page.getByTestId("session-name")).toHaveText(
        pilot.user.display_name,
      );
      await expect(page.getByRole("heading", { level: 1 })).toBeVisible();
      if (path === "/")
        await expect(page.getByTestId(`post-${pilot.post.id}`)).toBeVisible();
      if (path === "/settings/profile")
        await expect(
          page.getByRole("form", { name: "Modifica profilo" }),
        ).toBeVisible();
      await accessible(
        page,
        info,
        path.replaceAll("/", "_").replaceAll("?", "_") || "feed",
      );
    }
    await page.screenshot({
      path: info.outputPath(`settings-${info.project.name}.png`),
      fullPage: true,
    });
  });

  test("validazione profilo: focus sul campo errato, descrizione e testo conservato", async ({
    page,
  }, info) => {
    await page.goto("/settings/profile");
    const name = page.getByLabel("Nome visualizzato", { exact: true });
    const invalid = "a".repeat(51);
    await name.fill(invalid);
    await page
      .getByRole("button", { name: "Salva profilo", exact: true })
      .click();
    await expect(name).toBeFocused();
    await expect(name).toHaveAttribute("aria-invalid", "true");
    await expect(name).toHaveAccessibleDescription("Massimo 50 caratteri.");
    await expect(name).toHaveValue(invalid);
    await accessible(page, info, "profilo-errore");
  });
});
