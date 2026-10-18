import AxeBuilder from "@axe-core/playwright";
import type { Page, TestInfo } from "@playwright/test";
import { test, expect } from "./community-fixtures";

async function doubleText(page: Page) {
  await page.evaluate(() => {
    // Snapshot computed sizes before changing inheritance, as in Sprint 7.
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
}

async function checkLayout(page: Page, info: TestInfo, state: string) {
  const results = await new AxeBuilder({ page })
    .withTags(["wcag2a", "wcag2aa", "wcag21a", "wcag21aa", "wcag22aa"])
    .analyze();
  await info.attach(`community-layout-axe-${state}`, {
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
  expect
    .soft(
      await page.evaluate(
        () => document.documentElement.scrollWidth <= innerWidth,
      ),
      `${state}: nessuno scorrimento orizzontale della pagina`,
    )
    .toBe(true);

  const clipped: string[] = [];
  const outside: string[] = [];
  // Screen-reader-only labels intentionally use a clipped 1px box. They stay
  // in the full axe audit above; only visual geometry excludes that pattern.
  for (const element of await page
    .locator(
      "main h1, main h2, main p, main label:not(.sr-only), main summary, main button, main a, main input, main textarea",
    )
    .all()) {
    if (!(await element.isVisible())) continue;
    const geometry = await element.evaluate((node) => {
      const bounds = node.getBoundingClientRect();
      return {
        name: `${node.tagName}: ${(node.textContent || node.getAttribute("aria-label") || node.id).trim().slice(0, 90)}`,
        outside: bounds.left < -1 || bounds.right > innerWidth + 1,
        // Text inputs intentionally scroll their value horizontally; their
        // control must fit, while ordinary text must wrap without clipping.
        clipped:
          !(
            node instanceof HTMLInputElement ||
            node instanceof HTMLTextAreaElement
          ) && node.scrollWidth > node.clientWidth + 1,
      };
    });
    if (geometry.clipped) clipped.push(geometry.name);
    if (geometry.outside) outside.push(geometry.name);
  }
  expect
    .soft(outside, `${state}: testo e controlli dentro la viewport`)
    .toEqual([]);
  expect
    .soft(clipped, `${state}: testo leggibile senza taglio nel contenitore`)
    .toEqual([]);
  await page.screenshot({
    path: info.outputPath(`community-layout-${state}.png`),
    fullPage: true,
  });
}

test.describe("@auth @ux community: reflow con testo lungo", () => {
  test.skip(
    process.env.E2E_AUTH !== "1",
    "Richiede API, PostgreSQL e beta locale reali.",
  );

  test("nome Unicode di 60 caratteri, creazione e dettaglio a 320 px e testo al 200%", async ({
    page,
    context,
    circle,
    baseURL,
  }, info) => {
    test.setTimeout(180000);
    await context.addCookies(circle.owner.storage.cookies);
    const name = "🌿".repeat(30) + "S".repeat(30);
    expect(Array.from(name)).toHaveLength(60);
    const creation = await circle.owner.client.post("/api/v1/communities", {
      headers: { Origin: baseURL!, "X-CSRF-Protection": "1" },
      data: {
        name,
        description: "Descrizione".repeat(45),
        rules: "Rispetto".repeat(120),
      },
    });
    expect(creation.status()).toBe(201);
    const community = (await creation.json()).data.community as {
      id: string;
      name: string;
    };
    expect(community.name).toBe(name);

    for (const state of [
      { name: "320px", width: 320, double: false },
      { name: "320px-200pct", width: 320, double: true },
      {
        name: "viewport-200pct",
        width: Math.max(768, info.project.use.viewport?.width ?? 1440),
        double: true,
      },
    ]) {
      await page.setViewportSize({ width: state.width, height: 900 });
      await page.goto("/communities");
      await expect(
        page.getByRole("heading", { name, exact: true }),
      ).toBeVisible();
      await page
        .getByRole("button", { name: "Crea una community", exact: true })
        .click();
      const form = page.getByRole("form", {
        name: "Crea community",
        exact: true,
      });
      await form.getByLabel(/^Nome della community/).fill(name);
      await form
        .getByLabel(/^Descrizione/)
        .fill("Una community di prova con testo da leggere su ogni schermo.");
      await form
        .getByLabel(/^Regole della community/)
        .fill("Rispetto reciproco. Contenuti originali e pertinenti.");
      if (state.double) await doubleText(page);
      await checkLayout(page, info, `directory-${state.name}`);
      await expect(
        form.getByRole("button", { name: "Crea community", exact: true }),
      ).toBeVisible();

      await page.goto(`/communities/${community.id}`);
      await expect(
        page.getByRole("heading", { level: 1, name, exact: true }),
      ).toBeVisible();
      await expect(
        page.getByRole("form", { name: "Nuovo post", exact: true }),
      ).toBeVisible();
      await expect(
        page.getByRole("region", { name: "Post della community", exact: true }),
      ).toHaveAttribute("aria-busy", "false");
      await expect(
        page.getByRole("region", {
          name: "Membri della community",
          exact: true,
        }),
      ).toHaveAttribute("aria-busy", "false");
      if (state.double) await doubleText(page);
      await checkLayout(page, info, `detail-${state.name}`);
      await expect(
        page.getByRole("link", { name: "Tutte le community", exact: true }),
      ).toBeVisible();
    }
  });
});
