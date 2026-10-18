import { test as base, expect, type APIRequestContext } from "@playwright/test";
import { registerAccount } from "./helpers";
import type { User } from "../src/lib/api";
import type { Post } from "../src/lib/posts";

type Pilot = {
  user: User;
  post: Post;
  quote: Post;
  client: APIRequestContext;
  storage: Awaited<ReturnType<APIRequestContext["storageState"]>>;
};

export const test = base.extend<object, { pilot: Pilot }>({
  pilot: [
    async ({ playwright }, provide, workerInfo) => {
      const origin = String(
        workerInfo.project.use.baseURL ?? "http://localhost:3000",
      );
      if (!["http://localhost:3000", "https://localhost:3443"].includes(origin))
        throw new Error(
          "I collaudi UX con dati reali accettano solo la beta locale.",
        );
      const client = await playwright.request.newContext({
        baseURL: origin,
        ignoreHTTPSErrors: origin.startsWith("https:"),
      });
      const handle = `ux_${Date.now().toString(36)}_${workerInfo.workerIndex}`;
      const password = "Password di collaudo Sprint 7 🌿";
      const headers = { Origin: origin, "X-CSRF-Protection": "1" };
      let registered = false;
      try {
        const registration = await registerAccount(client, origin, {
          handle,
          email: `${handle}@example.test`,
          display_name: "Collaudo usabilità",
          password,
        });
        expect(registration.status()).toBe(201);
        registered = true;
        const user: User = (await registration.json()).data.user;
        const response = await client.post("/api/v1/posts", {
          headers,
          data: { body: "Messaggio di collaudo Sprint 7 🌿" },
        });
        expect(response.status()).toBe(201);
        const post: Post = (await response.json()).data.post;
        const quotation = await client.post("/api/v1/posts", {
          headers,
          data: {
            body: "Una seconda prospettiva di collaudo.",
            quote_of_id: post.id,
          },
        });
        expect(quotation.status()).toBe(201);
        const quote: Post = (await quotation.json()).data.post;
        await provide({
          user,
          post,
          quote,
          client,
          storage: await client.storageState(),
        });
      } finally {
        if (registered) {
          const removal = await client.delete("/api/v1/account", {
            headers,
            data: { password, confirmation: "ELIMINA" },
          });
          expect(
            removal.status(),
            "Rimuovere soltanto l’account sintetico creato da questa fixture",
          ).toBe(204);
        }
        await client.dispose();
      }
    },
    { scope: "worker" },
  ],
});
export { expect };
