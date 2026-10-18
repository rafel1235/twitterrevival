import AxeBuilder from "@axe-core/playwright";
import type { Page, TestInfo } from "@playwright/test";
import { test, expect } from "./community-fixtures";

async function checkAccessibility(page: Page, info: TestInfo, name: string) {
  const results = await new AxeBuilder({ page })
    .withTags(["wcag2a", "wcag2aa", "wcag21a", "wcag21aa", "wcag22aa"])
    .analyze();
  await info.attach(`community-axe-${name}`, {
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

async function publish(
  page: Page,
  label: "Nuovo post" | "Risposta",
  body: string,
) {
  const form = page.getByRole("form", { name: label, exact: true });
  await expect(form).toBeVisible();
  await form.getByRole("textbox").fill(body);
  const response = page.waitForResponse(
    (res) =>
      new URL(res.url()).pathname === "/api/v1/posts" &&
      res.request().method() === "POST",
  );
  await form
    .getByRole("button", {
      name: label === "Risposta" ? "Rispondi" : "Pubblica",
      exact: true,
    })
    .click();
  const received = await response;
  expect(received.status()).toBe(201);
  return (await received.json()).data.post as {
    id: string;
    community_id?: string;
  };
}

test("community e relativi contenuti richiedono una sessione", async ({
  page,
  request,
}) => {
  expect((await request.get("/api/v1/communities")).status()).toBe(401);
  for (const route of [
    "/communities",
    "/communities/00000000-0000-0000-0000-000000000000",
  ]) {
    await page.goto(route);
    await expect(page.getByRole("heading", { level: 1 })).toBeVisible();
    await expect(
      page.getByRole("link", { name: "Accedi", exact: true }).first(),
    ).toBeVisible();
    await expect(
      page.getByRole("form", { name: "Crea community", exact: true }),
    ).toHaveCount(0);
    await expect(
      page.getByRole("form", { name: "Nuovo post", exact: true }),
    ).toHaveCount(0);
  }
});

test.describe("@auth community con PostgreSQL reale", () => {
  test.skip(
    process.env.E2E_AUTH !== "1",
    "Richiede API, PostgreSQL e beta locale reali.",
  );
  test.beforeEach(async ({ context, circle }) => {
    await context.addCookies(circle.owner.storage.cookies);
  });

  test("creazione, scoperta, iscrizione, post, risposta e revoca dei contenuti all’uscita", async ({
    page,
    browser,
    baseURL,
    circle,
  }, info) => {
    test.setTimeout(180000);
    const origin = baseURL!;
    const headers = { Origin: origin, "X-CSRF-Protection": "1" };
    const groupName = `Sentieri Sprint 8 ${info.project.name}`;
    await page.goto("/communities");
    await page
      .getByRole("button", { name: "Crea una community", exact: true })
      .click();
    const create = page.getByRole("form", {
      name: "Crea community",
      exact: true,
    });
    await create.getByLabel(/^Nome della community/).fill(groupName);
    await create
      .getByLabel(/^Descrizione/)
      .fill("Un piccolo gruppo di collaudo con dati realmente salvati. 🌿");
    await create
      .getByLabel(/^Regole della community/)
      .fill("Rispetto reciproco e contenuti originali, senza spam.");
    const creation = page.waitForResponse(
      (res) =>
        new URL(res.url()).pathname === "/api/v1/communities" &&
        res.request().method() === "POST",
    );
    await create
      .getByRole("button", { name: "Crea community", exact: true })
      .click();
    const created = await creation;
    expect(created.status()).toBe(201);
    const community = (await created.json()).data.community as { id: string };
    await expect(page).toHaveURL(`/communities/${community.id}`);
    await expect(
      page.getByRole("heading", { name: groupName, exact: true }),
    ).toBeVisible();
    const ownerBody = `Sentieroriservato${info.project.name}: questa conversazione resta nel gruppo.`;
    const ownerPost = await publish(page, "Nuovo post", ownerBody);
    expect(ownerPost.community_id).toBe(community.id);
    await page.reload();
    const ownerCard = page.getByTestId(`post-${ownerPost.id}`);
    await expect(ownerCard).toContainText(ownerBody);
    await expect(
      ownerCard.getByRole("button", {
        name: /^(Ripubblica|Annulla repost|Cita),/,
      }),
    ).toHaveCount(0);
    await checkAccessibility(page, info, "owner");

    const memberContext = await browser.newContext({
      baseURL: origin,
      ignoreHTTPSErrors: origin === "https://localhost:3443",
    });
    try {
      await memberContext.addCookies(circle.member.storage.cookies);
      const memberPage = await memberContext.newPage();
      await memberPage.goto("/communities");
      await memberPage
        .getByRole("link")
        .filter({
          has: memberPage.getByRole("heading", {
            name: groupName,
            exact: true,
          }),
        })
        .click();
      await expect(
        memberPage.getByRole("heading", { name: groupName, exact: true }),
      ).toBeVisible();
      await expect(memberPage.getByTestId(`post-${ownerPost.id}`)).toHaveCount(
        0,
      );
      await expect(
        memberPage.getByRole("form", { name: "Nuovo post", exact: true }),
      ).toHaveCount(0);
      expect(
        (
          await circle.member.client.get(`/api/v1/posts/${ownerPost.id}`)
        ).status(),
      ).toBe(404);
      await checkAccessibility(memberPage, info, "non-member");
      const joining = memberPage.waitForResponse(
        (res) =>
          new URL(res.url()).pathname ===
            `/api/v1/communities/${community.id}/membership` &&
          res.request().method() === "POST",
      );
      await memberPage
        .getByRole("button", { name: "Iscriviti alla community", exact: true })
        .click();
      expect((await joining).status()).toBe(204);
      await expect(
        memberPage.getByTestId(`post-${ownerPost.id}`),
      ).toBeVisible();
      const memberBody = `Una scoperta riservata ai membri, ${info.project.name}. 🍂`;
      const memberPost = await publish(memberPage, "Nuovo post", memberBody);
      expect(memberPost.community_id).toBe(community.id);
      await memberPage.reload();
      await expect(
        memberPage.getByTestId(`post-${memberPost.id}`),
      ).toContainText(memberBody);
      await checkAccessibility(memberPage, info, "member");
      await memberPage.screenshot({
        path: info.outputPath(`community-${info.project.name}.png`),
        fullPage: true,
      });

      await page.goto(`/posts/${memberPost.id}`);
      const reply = await publish(
        page,
        "Risposta",
        `Rispondo nella stessa community, ${info.project.name}.`,
      );
      expect(reply.community_id).toBe(community.id);
      await expect(page.getByTestId(`post-${reply.id}`)).toBeVisible();
      for (const client of [circle.owner.client, circle.member.client]) {
        for (const route of [
          "/api/v1/feed/home",
          `/api/v1/users/${circle.owner.user.id}/posts`,
          `/api/v1/search?q=Sentieroriservato${info.project.name}`,
        ]) {
          const result = await client.get(route);
          expect(result.status()).toBe(200);
          const json = await result.text();
          for (const item of [ownerPost, memberPost, reply])
            expect(json).not.toContain(item.id);
        }
      }
      // UI ownership cannot grant global repost/quotation routes permission.
      expect(
        (
          await circle.member.client.post(
            `/api/v1/posts/${ownerPost.id}/reposts`,
            { headers, data: {} },
          )
        ).status(),
      ).toBe(403);

      await memberPage
        .getByRole("button", { name: "Abbandona community", exact: true })
        .click();
      const leaving = memberPage.waitForResponse(
        (res) =>
          new URL(res.url()).pathname ===
            `/api/v1/communities/${community.id}/membership` &&
          res.request().method() === "DELETE",
      );
      await memberPage
        .getByRole("button", { name: "Conferma uscita", exact: true })
        .click();
      expect((await leaving).status()).toBe(204);
      await expect(
        memberPage.getByRole("button", {
          name: "Iscriviti alla community",
          exact: true,
        }),
      ).toBeVisible();
      await expect(memberPage.getByTestId(`post-${ownerPost.id}`)).toHaveCount(
        0,
      );
      await expect(memberPage.getByTestId(`post-${memberPost.id}`)).toHaveCount(
        0,
      );
      await expect(
        memberPage.getByRole("form", { name: "Nuovo post", exact: true }),
      ).toHaveCount(0);
      for (const item of [ownerPost, memberPost, reply])
        expect(
          (await circle.member.client.get(`/api/v1/posts/${item.id}`)).status(),
        ).toBe(404);
      expect(
        (
          await circle.member.client.get(
            `/api/v1/communities/${community.id}/posts`,
          )
        ).status(),
      ).toBe(403);
      await memberPage.goto(`/posts/${memberPost.id}`);
      await expect(memberPage.getByTestId(`post-${memberPost.id}`)).toHaveCount(
        0,
      );
      await expect(
        memberPage.getByRole("form", { name: "Risposta", exact: true }),
      ).toHaveCount(0);
    } finally {
      await memberContext.close();
    }
  });

  test("modifica e archiviazione del titolare persistono e non espongono i post", async ({
    page,
    baseURL,
    circle,
  }, info) => {
    test.setTimeout(90000);
    const headers = { Origin: baseURL!, "X-CSRF-Protection": "1" };
    const created = await circle.owner.client.post("/api/v1/communities", {
      headers,
      data: {
        name: `Gestione ${info.project.name}`,
        description: "Community per collaudare la gestione.",
        rules: "Rispetto reciproco.",
      },
    });
    expect(created.status()).toBe(201);
    const community = (await created.json()).data.community as { id: string };
    await page.goto(`/communities/${community.id}`);
    const post = await publish(
      page,
      "Nuovo post",
      `Il post archiviato resta riservato, ${info.project.name}.`,
    );
    await page.getByText("Gestisci community", { exact: true }).click();
    await page
      .getByRole("button", { name: "Modifica community", exact: true })
      .click();
    const edit = page.getByRole("form", {
      name: "Modifica community",
      exact: true,
    });
    const renamed = `Nuovo nome ${info.project.name} 🌿`;
    await edit.getByLabel(/^Nome della community/).fill(renamed);
    await edit
      .getByLabel(/^Regole della community/)
      .fill("Regole aggiornate: cura e rispetto per ogni persona.");
    const update = page.waitForResponse(
      (res) =>
        new URL(res.url()).pathname === `/api/v1/communities/${community.id}` &&
        res.request().method() === "PATCH",
    );
    await edit
      .getByRole("button", { name: "Salva community", exact: true })
      .click();
    expect((await update).status()).toBe(200);
    await expect(
      page.getByRole("heading", { name: renamed, exact: true }),
    ).toBeVisible();
    await page.reload();
    await expect(
      page.getByRole("heading", { name: renamed, exact: true }),
    ).toBeVisible();
    await page.getByText("Gestisci community", { exact: true }).click();
    await page
      .getByRole("button", { name: "Archivia community", exact: true })
      .click();
    const archival = page.waitForResponse(
      (res) =>
        new URL(res.url()).pathname === `/api/v1/communities/${community.id}` &&
        res.request().method() === "DELETE",
    );
    await page
      .getByRole("button", { name: "Conferma archiviazione", exact: true })
      .click();
    expect((await archival).status()).toBe(204);
    await expect(
      page.getByRole("heading", { name: "Community archiviata", exact: true }),
    ).toBeVisible();
    await page
      .getByRole("link", { name: "Torna alle community", exact: true })
      .click();
    await expect(page).toHaveURL("/communities");
    expect(
      (
        await circle.owner.client.get(`/api/v1/communities/${community.id}`)
      ).status(),
    ).toBe(404);
    expect(
      (await circle.owner.client.get(`/api/v1/posts/${post.id}`)).status(),
    ).toBe(404);
    await page.reload();
    await expect(
      page.getByRole("heading", { name: renamed, exact: true }),
    ).toHaveCount(0);
  });
});
