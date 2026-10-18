import {
  expect,
  test,
  type APIRequestContext,
  type Page,
} from "@playwright/test";
import { registerAccount } from "./helpers";

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
    display_name:
      tag === "words_a" ? "Marta · Idee in circolo" : "Nico · Piccole scoperte",
    password: "post password sicura 🔐",
  });
  expect(response.status()).toBe(201);
  return (await response.json()).data.user;
}
async function create(client: APIRequestContext, origin: string, body: string) {
  const response = await client.post("/api/v1/posts", {
    headers: headers(origin),
    data: { body },
  });
  expect(response.status()).toBe(201);
  return (await response.json()).data.post;
}
async function publish(page: Page, action: () => Promise<void>) {
  const received = page.waitForResponse(
    (response) =>
      new URL(response.url()).pathname === "/api/v1/posts" &&
      response.request().method() === "POST",
  );
  await action();
  const response = await received;
  expect(response.status()).toBe(201);
  return (await response.json()).data.post;
}
test("post e conversazioni richiedono una sessione", async ({ page }) => {
  await page.goto("/posts/00000000-0000-0000-0000-000000000000");
  await expect(
    page.getByRole("heading", { name: "Entra nella community" }),
  ).toBeVisible();
  await expect(
    page.getByRole("form", { name: "Risposta", exact: true }),
  ).toHaveCount(0);
});

test("@auth post persistenti, feed, risposte, like, repost, citazioni e blocchi", async ({
  page,
  context,
  browser,
  baseURL,
}, testInfo) => {
  test.skip(process.env.E2E_AUTH !== "1", "Richiede API e PostgreSQL reali.");
  test.setTimeout(150000);
  const origin = baseURL!;
  const a = await register(context.request, origin, "words_a");
  const other = await browser.newContext({ baseURL: origin });
  try {
    const b = await register(other.request, origin, "words_b");
    const outside = await create(
      other.request,
      origin,
      "La luce del mattino cambia il modo in cui guardiamo le cose. Qual è la tua piccola scoperta di oggi? 🌿",
    );
    await page.goto("/");
    const composer = page.getByRole("form", {
      name: "Nuovo post",
      exact: true,
    });
    await expect(composer).toBeVisible();
    await expect(page.getByRole("article")).toHaveCount(0);
    await expect(page.getByRole("note")).toContainText("Nessun contenuto demo");
    await composer.getByRole("textbox").fill("🌿".repeat(281));
    await expect(
      composer.getByRole("button", { name: "Pubblica", exact: true }),
    ).toBeDisabled();
    await expect(composer.getByText("281/280", { exact: true })).toBeVisible();
    await composer.getByRole("textbox").fill("🌿".repeat(280) + "\uFEFF");
    await expect(
      composer.getByRole("button", { name: "Pubblica", exact: true }),
    ).toBeDisabled();
    await composer
      .getByRole("textbox")
      .fill("\u0085" + "🌿".repeat(280) + "\u0085");
    const own = await publish(page, () =>
      composer.getByRole("button", { name: "Pubblica", exact: true }).click(),
    );
    expect(own.body).toBe("🌿".repeat(280));
    const ownCard = page.getByTestId(`post-${own.id}`);
    await expect(ownCard).toBeVisible();
    await page.reload();
    await expect(ownCard).toBeVisible();
    await ownCard
      .getByRole("button", { name: "Modifica", exact: true })
      .click();
    const editing = ownCard.getByRole("form", {
      name: "Modifica post",
      exact: true,
    });
    const edited =
      'Caffè, idee e accenti: è già una buona partenza. 🌿 <img src=x onerror="alert(1)">';
    await editing.getByRole("textbox").fill(edited);
    await editing.getByRole("button", { name: "Salva modifica" }).click();
    await expect(ownCard.getByText(edited, { exact: true })).toBeVisible();
    await expect(ownCard.locator("img")).toHaveCount(0);
    await page.goto(`/u/${b.handle}`);
    await page.getByRole("button", { name: "Segui", exact: true }).click();
    await expect(
      page.getByRole("button", { name: "Non seguire più", exact: true }),
    ).toBeVisible();
    await page.goto("/");
    const otherCard = page.getByTestId(`post-${outside.id}`).first();
    await expect(otherCard).toBeVisible();
    await otherCard
      .getByRole("button", { name: "Mi piace", exact: true })
      .click();
    await expect(
      otherCard.getByRole("button", { name: "Rimuovi mi piace", exact: true }),
    ).toHaveAttribute("aria-pressed", "true");
    await page.reload();
    await expect(
      otherCard.getByRole("button", { name: "Rimuovi mi piace", exact: true }),
    ).toBeVisible();
    await otherCard
      .getByRole("button", { name: "Ripubblica", exact: true })
      .click();
    await expect(
      page.getByText(`Ripubblicato da @${a.handle}`, { exact: true }),
    ).toBeVisible();
    await otherCard
      .getByRole("button", { name: "Annulla repost", exact: true })
      .click();
    await expect(
      page.getByText(`Ripubblicato da @${a.handle}`, { exact: true }),
    ).toHaveCount(0);
    await page.goto(`/posts/${outside.id}`);
    const root = page.getByTestId(`post-${outside.id}`);
    await root.getByRole("button", { name: "Cita", exact: true }).click();
    const citing = root.getByRole("form", { name: "Citazione", exact: true });
    await citing
      .getByRole("textbox")
      .fill(
        "Una domanda da portare con sé. Oggi riscopro il piacere di ascoltare. ☕",
      );
    const quote = await publish(page, () =>
      citing
        .getByRole("button", { name: "Pubblica citazione", exact: true })
        .click(),
    );
    const replyForm = page.getByRole("form", { name: "Risposta", exact: true });
    await replyForm
      .getByRole("textbox")
      .fill("Ho scoperto un nuovo sentiero, proprio vicino a casa. 🍂");
    const reply = await publish(page, () =>
      replyForm.getByRole("button", { name: "Rispondi", exact: true }).click(),
    );
    await expect(page.getByTestId(`post-${reply.id}`)).toBeVisible();
    await page.reload();
    await expect(page.getByTestId(`post-${reply.id}`)).toBeVisible();
    expect(
      await page.evaluate(
        () => document.documentElement.scrollWidth <= window.innerWidth,
      ),
    ).toBe(true);
    await page.screenshot({
      path: testInfo.outputPath(`conversation-${testInfo.project.name}.png`),
      fullPage: true,
    });
    await page.goto(`/posts/${quote.id}`);
    await expect(
      page.getByRole("link", {
        name: `Citazione di @${b.handle}`,
        exact: true,
      }),
    ).toContainText(outside.body);
    if (testInfo.project.name === "desktop") {
      for (let i = 0; i < 21; i++)
        await create(
          other.request,
          origin,
          `Pensiero numero ${i + 1}: le conversazioni crescono, un passo alla volta.`,
        );
      await page.goto("/");
      await expect(page.getByRole("article")).toHaveCount(20);
      await page
        .getByRole("button", { name: "Carica altri post", exact: true })
        .click();
      await expect(ownCard).toBeVisible();
      const firstPageIds = await page
        .getByRole("article")
        .evaluateAll((articles) =>
          articles.map((article) => article.getAttribute("data-testid")),
        );
      expect(new Set(firstPageIds).size).toBe(firstPageIds.length);
    }
    await page.goto(`/u/${b.handle}`);
    await page.getByRole("button", { name: "Blocca", exact: true }).click();
    await page
      .getByRole("button", { name: "Conferma blocco", exact: true })
      .click();
    await expect(page).toHaveURL("/settings/blocks");
    expect(
      (await context.request.get(`/api/v1/posts/${outside.id}`)).status(),
    ).toBe(404);
    await page.goto("/");
    await expect(ownCard).toBeVisible();
    await expect(page.getByText(outside.body, { exact: true })).toHaveCount(0);
    await expect(
      page.getByText("Post originale non disponibile.", { exact: true }),
    ).toBeVisible();
    await ownCard.getByRole("button", { name: "Elimina", exact: true }).click();
    await ownCard
      .getByRole("button", { name: "Conferma eliminazione", exact: true })
      .click();
    await expect(ownCard).toHaveCount(0);
    expect(
      (await context.request.get(`/api/v1/posts/${own.id}`)).status(),
    ).toBe(404);
    await page.reload();
    await expect(ownCard).toHaveCount(0);
    await expect(page.getByTestId(`post-${quote.id}`)).toBeVisible();
    expect(
      await page.evaluate(
        () => document.documentElement.scrollWidth <= window.innerWidth,
      ),
    ).toBe(true);
    await page.screenshot({
      path: testInfo.outputPath(`timeline-${testInfo.project.name}.png`),
      fullPage: true,
    });
  } finally {
    await other.close();
  }
});
