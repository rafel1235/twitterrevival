import { test as base, expect, type APIRequestContext } from "@playwright/test";
import { registerAccount } from "./helpers";
import type { User } from "../src/lib/api";

type CommunityActor = {
  user: User;
  client: APIRequestContext;
  storage: Awaited<ReturnType<APIRequestContext["storageState"]>>;
};

type Circle = {
  owner: CommunityActor;
  member: CommunityActor;
};

export const test = base.extend<object, { circle: Circle }>({
  circle: [
    async ({ playwright }, provide, worker) => {
      const origin = String(
        worker.project.use.baseURL ?? "http://localhost:3000",
      );
      if (!["http://localhost:3000", "https://localhost:3443"].includes(origin))
        throw new Error(
          "Il collaudo community con dati reali accetta solo la beta locale.",
        );
      const password = "Password di collaudo community Sprint 8 🌿";
      const actors: CommunityActor[] = [];
      const clients: APIRequestContext[] = [];
      try {
        for (const role of ["o", "m"]) {
          const client = await playwright.request.newContext({
            baseURL: origin,
            ignoreHTTPSErrors: origin === "https://localhost:3443",
          });
          clients.push(client);
          const handle = `c8${role}_${Date.now().toString(36)}_${worker.workerIndex}`;
          const registration = await registerAccount(client, origin, {
            handle,
            email: `${handle}@example.test`,
            display_name: role === "o" ? "Ada · Community" : "Nico · Community",
            password,
          });
          expect(registration.status()).toBe(201);
          actors.push({
            user: (await registration.json()).data.user,
            client,
            storage: await client.storageState(),
          });
        }
        await provide({ owner: actors[0], member: actors[1] });
      } finally {
        // Only the two synthetic accounts created above are erased. Erasure of
        // the owner also archives their communities; no application-wide purge.
        for (const actor of actors.reverse()) {
          const removal = await actor.client.delete("/api/v1/account", {
            headers: { Origin: origin, "X-CSRF-Protection": "1" },
            data: { password, confirmation: "ELIMINA" },
          });
          expect(
            removal.status(),
            "Rimozione del solo account sintetico della fixture community",
          ).toBe(204);
        }
        for (const client of clients) await client.dispose();
      }
    },
    { scope: "worker" },
  ],
});

export { expect };
