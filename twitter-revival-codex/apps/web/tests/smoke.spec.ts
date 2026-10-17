import { expect, test } from "@playwright/test";

test("home responsive, demo esplicita e composer disabilitato", async ({
  page,
}, testInfo) => {
  await page.goto("/");
  await expect(
    page.getByRole("heading", { name: "Home", exact: true }),
  ).toBeVisible();
  await expect(page.getByRole("note")).toContainText(
    "Feed demo con persone e post inventati",
  );
  await expect(
    page.getByRole("button", { name: "Pubblica", exact: true }),
  ).toBeDisabled();
  await expect(
    page.getByRole("textbox", { name: "Nuovo post" }),
  ).toBeDisabled();
  await expect(page.getByRole("article")).toHaveCount(3);
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= window.innerWidth,
    ),
  ).toBe(true);
  if (testInfo.project.name === "mobile")
    await expect(
      page.getByRole("navigation", { name: "Navigazione mobile" }),
    ).toBeVisible();
  await page.screenshot({
    path: testInfo.outputPath(`home-${testInfo.project.name}.png`),
    fullPage: true,
  });
});

test("form accessibili e navigazione login/registrazione", async ({ page }) => {
  await page.goto("/login");
  await expect(page.getByRole("form", { name: "Accesso" })).toBeVisible();
  await expect(page.getByLabel("Email", { exact: true })).toBeVisible();
  await page.getByRole("link", { name: "Registrati", exact: true }).click();
  await expect(page.getByRole("form", { name: "Registrazione" })).toBeVisible();
  await expect(page.getByLabel("Handle", { exact: true })).toBeVisible();
});

test("@auth registrazione, refresh, logout e login su PostgreSQL reale", async ({
  page,
  context,
  request,
}, testInfo) => {
  test.skip(
    process.env.E2E_AUTH !== "1",
    "Richiede API e PostgreSQL reali; impostare E2E_AUTH=1.",
  );
  const handle = `e2e_${Date.now().toString(36)}_${testInfo.project.name.slice(0, 1)}`;
  const email = `${handle}@example.test`;
  const password = "e2e password lunga 🔐";
  const name = `Persona ${testInfo.project.name}`;
  await page.goto("/register");
  await page.getByLabel("Come ti chiami?").fill(name);
  await page.getByLabel("Handle", { exact: true }).fill(handle);
  await page.getByLabel("Email", { exact: true }).fill(email);
  await page.getByLabel("Password", { exact: true }).fill(password);
  await page.getByRole("button", { name: "Crea account", exact: true }).click();
  await expect(page).toHaveURL("/");
  await expect(page.getByTestId("session-name")).toHaveText(name);
  const cookie = (await context.cookies()).find((value) =>
    value.name.endsWith("radura_session"),
  );
  expect(cookie?.httpOnly).toBe(true);
  expect(cookie?.sameSite).toBe("Lax");
  await page.reload();
  await expect(page.getByTestId("session-name")).toHaveText(name);
  await page.getByRole("button", { name: "Esci", exact: true }).click();
  await expect(
    page.getByRole("button", { name: "Esci", exact: true }),
  ).toHaveCount(0);
  expect((await request.get("/api/v1/auth/me")).status()).toBe(401);
  await page.goto("/login");
  await page.getByLabel("Email", { exact: true }).fill(email);
  await page.getByLabel("Password", { exact: true }).fill("a wrong password");
  await page.getByRole("button", { name: "Accedi", exact: true }).click();
  await expect(
    page.getByRole("form", { name: "Accesso" }).getByRole("alert"),
  ).toContainText("Credenziali");
  await page.getByLabel("Password", { exact: true }).fill(password);
  await page.getByRole("button", { name: "Accedi", exact: true }).click();
  await expect(page).toHaveURL("/");
  await expect(page.getByTestId("session-name")).toHaveText(name);
  await page.screenshot({
    path: testInfo.outputPath(`authenticated-${testInfo.project.name}.png`),
    fullPage: true,
  });
  await page.getByRole("button", { name: "Esci", exact: true }).click();
  await expect(
    page.getByRole("button", { name: "Esci", exact: true }),
  ).toHaveCount(0);
});
