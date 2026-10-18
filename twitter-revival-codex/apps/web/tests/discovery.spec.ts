import { expect, test, type APIRequestContext } from "@playwright/test";
import { resolve } from "node:path";
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
    display_name: tag === "media_a" ? "Lina Riva" : "Elena Bosco",
    password: "media password sicura 🔐",
  });
  expect(response.status()).toBe(201);
  return (await response.json()).data.user;
}
test("ricerca, notifiche e immagini richiedono accesso", async ({
  page,
  request,
}) => {
  for (const path of ["/search", "/notifications"]) {
    await page.goto(path);
    await expect(
      page.getByRole("heading", { name: "Entra nella community" }),
    ).toBeVisible();
  }
  expect(
    (
      await request.get(
        "/api/v1/media/00000000-0000-0000-0000-000000000000/content",
      )
    ).status(),
  ).toBe(401);
});
test("@auth immagini S3, ricerca e notifiche persistenti con blocchi", async ({
  page,
  context,
  browser,
  baseURL,
}, testInfo) => {
  test.skip(
    process.env.E2E_AUTH !== "1",
    "Richiede API, PostgreSQL e storage S3 reali.",
  );
  test.setTimeout(150000);
  const origin = baseURL!;
  const a = await register(context.request, origin, "media_a");
  const other = await browser.newContext({ baseURL: origin });
  try {
    const b = await register(other.request, origin, "media_b");
    const term = `rugiada${Date.now().toString(36)}`;
    const alt = "Paesaggio geometrico originale: colline verdi e sole";
    await page.goto("/");
    let composer = page.getByRole("form", { name: "Nuovo post", exact: true });
    await composer
      .getByRole("button", { name: "Aggiungi immagine (0/4)", exact: true })
      .click();
    const upload = composer.getByRole("group", { name: "Allega immagine" });
    await upload
      .getByLabel("Immagine JPEG o PNG", { exact: true })
      .setInputFiles({
        name: "mascherata.png",
        mimeType: "image/png",
        buffer: Buffer.from('<svg onload="alert(1)"/>'),
      });
    await upload
      .getByLabel("Descrizione dell’immagine", { exact: true })
      .fill(alt);
    await upload
      .getByRole("button", { name: "Carica immagine", exact: true })
      .click();
    await expect(composer.getByRole("alert")).toContainText(
      "JPEG o PNG valide",
    );
    await upload
      .getByLabel("Immagine JPEG o PNG", { exact: true })
      .setInputFiles(resolve("apps/web/tests/fixtures/radura-landscape.png"));
    await upload
      .getByRole("button", { name: "Carica immagine", exact: true })
      .click();
    await expect(
      composer.getByRole("img", { name: alt, exact: true }),
    ).toHaveJSProperty("naturalWidth", 600);
    const drafts = await context.request.get("/api/v1/media/drafts");
    expect(drafts.status()).toBe(200);
    const image = (await drafts.json()).data.media[0];
    expect((await other.request.get(image.url)).status()).toBe(404);
    await page.reload();
    composer = page.getByRole("form", { name: "Nuovo post", exact: true });
    await composer
      .getByRole("button", { name: "Aggiungi immagine (0/4)", exact: true })
      .click();
    await composer
      .getByRole("button", { name: "Usa immagine", exact: true })
      .click();
    await expect(
      composer.getByRole("img", { name: alt, exact: true }),
    ).toBeVisible();
    await composer
      .getByRole("textbox", { name: "Nuovo post", exact: true })
      .fill(`${term} Un mattino tra colline e nuove idee. 🌿`);
    const published = page.waitForResponse(
      (response) =>
        new URL(response.url()).pathname === "/api/v1/posts" &&
        response.request().method() === "POST",
    );
    await composer
      .getByRole("button", { name: "Pubblica", exact: true })
      .click();
    const postResponse = await published;
    expect(postResponse.status()).toBe(201);
    const post = (await postResponse.json()).data.post;
    const card = page.getByTestId(`post-${post.id}`);
    await expect(card.getByRole("img", { name: alt })).toHaveJSProperty(
      "naturalWidth",
      600,
    );
    await page.reload();
    await expect(card.getByRole("img", { name: alt })).toBeVisible();
    expect((await other.request.get(image.url)).status()).toBe(200);
    await page.goto("/search");
    const search = page.getByRole("search", { name: "Ricerca", exact: true });
    await search.getByLabel("Cerca in Radura", { exact: true }).fill(term);
    await search.getByRole("button", { name: "Cerca", exact: true }).click();
    await expect(page.getByTestId(`post-${post.id}`)).toBeVisible();
    expect(
      await page.evaluate(
        () => document.documentElement.scrollWidth <= window.innerWidth,
      ),
    ).toBe(true);
    await page.screenshot({
      path: testInfo.outputPath(`search-media-${testInfo.project.name}.png`),
      fullPage: true,
    });
    await page
      .getByRole("navigation", { name: "Tipo di ricerca" })
      .getByRole("link", { name: "Persone", exact: true })
      .click();
    await page
      .getByRole("search", { name: "Ricerca", exact: true })
      .getByLabel("Cerca in Radura", { exact: true })
      .fill(b.handle);
    await page
      .getByRole("search", { name: "Ricerca", exact: true })
      .getByRole("button", { name: "Cerca", exact: true })
      .click();
    await expect(
      page
        .getByRole("region", { name: "Risultati persone" })
        .getByRole("link", { name: b.display_name, exact: true }),
    ).toBeVisible();
    for (const path of [
      `/users/${a.id}/follow`,
      `/posts/${post.id}/likes`,
      `/posts/${post.id}/reposts`,
    ])
      expect(
        (
          await other.request.post(`/api/v1${path}`, {
            headers: headers(origin),
            data: {},
          })
        ).status(),
      ).toBe(path.includes("follow") ? 204 : 200);
    for (const ref of ["reply_to_id", "quote_of_id"]) {
      const response = await other.request.post("/api/v1/posts", {
        headers: headers(origin),
        data: {
          body:
            ref === "reply_to_id"
              ? "Che bel paesaggio, mi ricorda casa. 🌱"
              : "Una prospettiva da condividere.",
          [ref]: post.id,
        },
      });
      expect(response.status()).toBe(201);
    }
    await page.goto("/notifications");
    await expect(
      page
        .getByRole("region", { name: "Elenco notifiche" })
        .getByRole("article"),
    ).toHaveCount(5);
    await expect(
      page.getByText("5 da leggere · Aggiornamento ogni 30 secondi", {
        exact: true,
      }),
    ).toBeVisible();
    await page
      .getByRole("button", { name: "Segna come letta", exact: true })
      .first()
      .click();
    await expect(
      page.getByText("4 da leggere · Aggiornamento ogni 30 secondi", {
        exact: true,
      }),
    ).toBeVisible();
    await page.reload();
    await expect(
      page.getByText("4 da leggere · Aggiornamento ogni 30 secondi", {
        exact: true,
      }),
    ).toBeVisible();
    expect(
      await page.evaluate(
        () => document.documentElement.scrollWidth <= window.innerWidth,
      ),
    ).toBe(true);
    await page.screenshot({
      path: testInfo.outputPath(`notifications-${testInfo.project.name}.png`),
      fullPage: true,
    });
    await page
      .getByRole("button", { name: "Segna tutte come lette", exact: true })
      .click();
    await expect(
      page.getByText("0 da leggere · Aggiornamento ogni 30 secondi", {
        exact: true,
      }),
    ).toBeVisible();
    if (testInfo.project.name === "desktop") {
      await page.clock.install();
      await page.reload();
      await expect(
        page.getByText("0 da leggere · Aggiornamento ogni 30 secondi", {
          exact: true,
        }),
      ).toBeVisible();
      const incoming = await other.request.post("/api/v1/posts", {
        headers: headers(origin),
        data: {
          body: "Una nuova risposta arrivata mentre leggi.",
          reply_to_id: post.id,
        },
      });
      expect(incoming.status()).toBe(201);
      await page.clock.fastForward(31000);
      await expect(
        page.getByText("1 da leggere · Aggiornamento ogni 30 secondi", {
          exact: true,
        }),
      ).toBeVisible();
      await page.clock.resume();
    }
    expect(
      (
        await context.request.post(`/api/v1/users/${b.id}/block`, {
          headers: headers(origin),
          data: {},
        })
      ).status(),
    ).toBe(204);
    await page
      .getByRole("button", { name: "Aggiorna notifiche", exact: true })
      .click();
    await expect(
      page
        .getByRole("region", { name: "Elenco notifiche" })
        .getByRole("article"),
    ).toHaveCount(0);
    expect((await other.request.get(image.url)).status()).toBe(404);
    const hidden = await other.request.get(`/api/v1/search?q=${term}`);
    expect(hidden.status()).toBe(200);
    expect((await hidden.json()).data.items).toEqual([]);
    await page.goto(`/search?q=${b.handle}&type=users`);
    await expect(
      page.getByText("Nessuna persona visibile corrisponde alla ricerca.", {
        exact: true,
      }),
    ).toBeVisible();
    expect(
      (
        await context.request.delete(`/api/v1/posts/${post.id}`, {
          headers: headers(origin),
          data: {},
        })
      ).status(),
    ).toBe(204);
    expect((await context.request.get(image.url)).status()).toBe(404);
  } finally {
    await other.close();
  }
});
