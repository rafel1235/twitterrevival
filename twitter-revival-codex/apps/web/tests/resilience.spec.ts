import { test, expect } from "./ux-fixtures";
import { registerAccount } from "./helpers";
import AxeBuilder from "@axe-core/playwright";

test("@ux sessione non verificabile: errore recuperabile senza feed demo", async ({
  page,
}) => {
  await page.route("**/api/v1/auth/me", (route) => route.abort("failed"));
  await page.goto("/");
  const main = page.getByRole("main");
  await expect(main.getByRole("alert")).toContainText(
    "Impossibile verificare la sessione",
  );
  await expect(main.getByRole("note")).toHaveCount(0);
  await page.unroute("**/api/v1/auth/me");
  await main.getByRole("button", { name: "Riprova", exact: true }).click();
  await expect(main.getByRole("note")).toContainText("Feed demo");
});

test.describe("@auth @ux recupero con dati reali", () => {
  test.skip(
    process.env.E2E_AUTH !== "1",
    "Richiede la beta locale con API e database reali.",
  );
  test.beforeEach(async ({ context, pilot }) => {
    await context.addCookies(pilot.storage.cookies);
  });

  test("feed conservato dopo 503 e recuperato con retry", async ({
    page,
    pilot,
  }) => {
    await page.goto("/");
    const feed = page.getByRole("region", { name: "Timeline dei seguiti" });
    await expect(feed.getByTestId(`post-${pilot.post.id}`)).toBeVisible();
    await page.route("**/api/v1/feed/home?*", (route) =>
      route.fulfill({
        status: 503,
        contentType: "text/html",
        body: "<html>proxy unavailable</html>",
      }),
    );
    await feed.getByRole("button", { name: "Aggiorna", exact: true }).click();
    await expect(feed.getByRole("alert")).toContainText(
      "Servizio non disponibile",
    );
    await expect(feed.getByTestId(`post-${pilot.post.id}`)).toBeVisible();
    await page.unroute("**/api/v1/feed/home?*");
    await feed.getByRole("button", { name: "Riprova", exact: true }).click();
    await expect(feed.getByRole("alert")).toHaveCount(0);
    await expect(feed.getByTestId(`post-${pilot.post.id}`)).toBeVisible();
    await page.route("**/api/v1/feed/home?*", (route) =>
      route.fulfill({
        status: 403,
        json: { error: { message: "Accesso negato." } },
      }),
    );
    await feed.getByRole("button", { name: "Aggiorna", exact: true }).click();
    await expect(feed.getByRole("alert")).toContainText("Accesso negato");
    await expect(feed.getByTestId(`post-${pilot.post.id}`)).toHaveCount(0);
  });

  test("notifiche conservate durante un guasto transitorio", async ({
    page,
    pilot,
    playwright,
    baseURL,
  }) => {
    const origin = baseURL!;
    const actor = await playwright.request.newContext({
      baseURL: origin,
      ignoreHTTPSErrors: origin.startsWith("https:"),
    });
    const handle = `notice_${Date.now().toString(36)}`;
    const password = "Password collaudo notifiche 🌿";
    const headers = { Origin: origin, "X-CSRF-Protection": "1" };
    let registered = false;
    try {
      const registration = await registerAccount(actor, origin, {
        handle,
        email: `${handle}@example.test`,
        display_name: "Persona notifiche",
        password,
      });
      expect(registration.status()).toBe(201);
      registered = true;
      expect(
        (
          await actor.post(`/api/v1/users/${pilot.user.id}/follow`, {
            headers,
            data: {},
          })
        ).status(),
      ).toBe(204);
      await page.goto("/notifications");
      const notification = page.getByRole("article", {
        name: `Notifica: @${handle} ha iniziato a seguirti`,
      });
      await expect(notification).toBeVisible();
      expect(
        (
          await new AxeBuilder({ page })
            .withTags(["wcag2a", "wcag2aa", "wcag21a", "wcag21aa", "wcag22aa"])
            .analyze()
        ).violations,
      ).toEqual([]);
      await page.route("**/api/v1/notifications?*", (route) =>
        route.fulfill({
          status: 503,
          json: { error: { message: "Aggiornamento non disponibile." } },
        }),
      );
      await page
        .getByRole("button", { name: "Aggiorna notifiche", exact: true })
        .click();
      await expect(page.getByRole("main").getByRole("alert")).toContainText(
        "Aggiornamento non disponibile",
      );
      await expect(notification).toBeVisible();
      await page.unroute("**/api/v1/notifications?*");
      await page
        .getByRole("main")
        .getByRole("button", { name: "Riprova", exact: true })
        .click();
      await expect(page.getByRole("main").getByRole("alert")).toHaveCount(0);
      await expect(notification).toBeVisible();
    } finally {
      if (registered)
        expect(
          (
            await actor.delete("/api/v1/account", {
              headers,
              data: { password, confirmation: "ELIMINA" },
            })
          ).status(),
        ).toBe(204);
      await actor.dispose();
    }
  });

  test("retry della pagina fallita conserva cursore e post precedenti", async ({
    page,
    pilot,
  }) => {
    const cursors: string[] = [];
    await page.route("**/api/v1/feed/home?*", async (route) => {
      const cursor = new URL(route.request().url()).searchParams.get("cursor");
      if (!cursor) {
        const response = await route.fetch();
        const payload = await response.json();
        // Only pagination metadata is synthetic; the visible posts come from PostgreSQL.
        payload.data.next_cursor = "ux-test-page";
        await route.fulfill({ response, json: payload });
      } else {
        cursors.push(cursor);
        await route.fulfill(
          cursors.length === 1
            ? {
                status: 503,
                json: {
                  error: { message: "Pagina temporaneamente non disponibile." },
                },
              }
            : { json: { data: { items: [] } } },
        );
      }
    });
    await page.goto("/");
    const feed = page.getByRole("region", { name: "Timeline dei seguiti" });
    await expect(feed.getByTestId(`post-${pilot.post.id}`)).toBeVisible();
    await feed
      .getByRole("button", { name: "Carica altri post", exact: true })
      .click();
    await expect(feed.getByRole("alert")).toContainText(
      "Pagina temporaneamente",
    );
    await expect(feed.getByTestId(`post-${pilot.post.id}`)).toBeVisible();
    await feed.getByRole("button", { name: "Riprova", exact: true }).click();
    await expect(feed.getByRole("alert")).toHaveCount(0);
    expect(cursors).toEqual(["ux-test-page", "ux-test-page"]);
    await expect(feed.getByTestId(`post-${pilot.post.id}`)).toHaveCount(1);
    await expect(
      feed.getByRole("button", { name: "Carica altri post", exact: true }),
    ).toHaveCount(0);
  });

  test("pubblicazione interrotta: testo conservato e nessun retry automatico", async ({
    page,
    pilot,
  }) => {
    await page.goto("/");
    const composer = page.getByRole("form", {
      name: "Nuovo post",
      exact: true,
    });
    const body = `Bozza dopo errore di rete ${Date.now()} 🌿`;
    await composer.getByRole("textbox").fill(body);
    let writes = 0;
    await page.route("**/api/v1/posts", (route) => {
      if (route.request().method() !== "POST") return route.continue();
      writes++;
      return route.abort("failed");
    });
    await composer
      .getByRole("button", { name: "Pubblica", exact: true })
      .click();
    await expect(composer.getByRole("alert")).toContainText(
      "L’operazione potrebbe essere riuscita",
    );
    await expect(composer.getByRole("textbox")).toHaveValue(body);
    await expect(
      composer.getByRole("button", { name: "Pubblica", exact: true }),
    ).toBeEnabled();
    expect(writes).toBe(1);
    await page.unroute("**/api/v1/posts");
    await composer
      .getByRole("button", { name: "Pubblica", exact: true })
      .click();
    await expect(composer.getByRole("status")).toHaveText("Post pubblicato.");
    const feed = await pilot.client.get("/api/v1/feed/home?limit=20");
    const items = (await feed.json()).data.items as {
      post: { body: string };
    }[];
    expect(items.filter((item) => item.post.body === body)).toHaveLength(1);
  });

  test("refresh concorrenti: la richiesta precedente viene annullata", async ({
    page,
    pilot,
  }) => {
    const path = `**/api/v1/posts/${pilot.post.id}`;
    await page.goto(`/posts/${pilot.post.id}`);
    await expect(page.getByTestId(`post-${pilot.post.id}`)).toBeVisible();
    let entered!: () => void;
    let release!: () => void;
    const started = new Promise<void>((resolve) => {
      entered = resolve;
    });
    const held = new Promise<void>((resolve) => {
      release = resolve;
    });
    let calls = 0;
    await page.route(path, async (route) => {
      calls++;
      if (calls === 1) {
        entered();
        await held;
        await route.fulfill({
          status: 503,
          json: { error: { message: "Risposta obsoleta del collaudo" } },
        });
      } else await route.continue();
    });
    try {
      await page.evaluate(() =>
        window.dispatchEvent(new Event("radura:posts-changed")),
      );
      await started;
      const aborted = page.waitForEvent(
        "requestfailed",
        (request) =>
          new URL(request.url()).pathname === `/api/v1/posts/${pilot.post.id}`,
      );
      await page.evaluate(() =>
        window.dispatchEvent(new Event("radura:posts-changed")),
      );
      await aborted;
      release();
      await expect(page.getByTestId(`post-${pilot.post.id}`)).toBeVisible();
      await expect(
        page.getByText("Risposta obsoleta del collaudo"),
      ).toHaveCount(0);
    } finally {
      release();
      await page.unrouteAll({ behavior: "wait" });
    }
  });

  test("sessione scaduta: avviso esplicito e contenuti privati rimossi", async ({
    page,
    pilot,
  }, info) => {
    await page.goto("/");
    await expect(page.getByTestId(`post-${pilot.post.id}`)).toBeVisible();
    await page.route("**/api/v1/notifications?*", (route) =>
      route.fulfill({
        status: 401,
        json: { error: { message: "Sessione scaduta." } },
      }),
    );
    const nav = page.getByRole("navigation", {
      name:
        info.project.name === "mobile"
          ? "Navigazione mobile"
          : "Navigazione principale",
    });
    await nav.getByRole("link", { name: "Notifiche", exact: true }).click();
    await expect(page.getByRole("main").getByRole("alert")).toContainText(
      "La sessione è scaduta o è stata revocata",
    );
    await expect(page.getByTestId("session-name")).toHaveCount(0);
    await expect(page.getByTestId(`post-${pilot.post.id}`)).toHaveCount(0);
    await expect(
      page.getByRole("link", { name: "Accedi di nuovo", exact: true }),
    ).toBeVisible();
  });

  test("errore di rendering isolato alla pagina e recupero", async ({
    page,
    pilot,
  }) => {
    await page.route("**/api/v1/feed/home?*", (route) =>
      route.fulfill({ json: { data: null } }),
    );
    await page.goto("/");
    await expect(
      page.getByRole("heading", { name: "Questa pagina non si è caricata" }),
    ).toBeVisible();
    await expect(
      page.getByRole("link", { name: "Vai al contenuto" }),
    ).toBeAttached();
    await page.unroute("**/api/v1/feed/home?*");
    await page
      .getByRole("button", { name: "Riprova pagina", exact: true })
      .click();
    await expect(page.getByTestId(`post-${pilot.post.id}`)).toBeVisible();
  });
});
