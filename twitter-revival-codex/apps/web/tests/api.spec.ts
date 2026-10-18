import { expect, test } from "@playwright/test";
import { api, ApiError } from "../src/lib/api";

const originalFetch = globalThis.fetch;
test.afterEach(() => {
  globalThis.fetch = originalFetch;
});

test("GET same-origin, senza cache e con cookie", async () => {
  let request: RequestInit | undefined;
  let url: unknown;
  globalThis.fetch = async (input, init) => {
    url = input;
    request = init;
    return Response.json({ data: { ok: true } });
  };
  await expect(api("/beta")).resolves.toEqual({ data: { ok: true } });
  expect(url).toBe("/api/v1/beta");
  expect(request).toMatchObject({
    method: "GET",
    credentials: "same-origin",
    cache: "no-store",
  });
  expect(request?.headers).toBeUndefined();
});

test("mutazioni JSON con protezione CSRF e risposta 204", async () => {
  let request: RequestInit | undefined;
  globalThis.fetch = async (_, init) => {
    request = init;
    return new Response(null, { status: 204 });
  };
  await expect(
    api("/users/me", { method: "PATCH", body: { bio: "🌿" } }),
  ).resolves.toBeUndefined();
  expect(request).toMatchObject({
    method: "PATCH",
    body: JSON.stringify({ bio: "🌿" }),
    headers: { "Content-Type": "application/json", "X-CSRF-Protection": "1" },
  });
});

test("upload multipart senza Content-Type manuale e download blob", async () => {
  let request: RequestInit | undefined;
  const form = new FormData();
  form.set("alt_text", "Un paesaggio");
  globalThis.fetch = async (_, init) => {
    request = init;
    return Response.json({ data: {} });
  };
  await api("/media", { form });
  expect(request?.body).toBe(form);
  expect(request?.headers).toEqual({ "X-CSRF-Protection": "1" });
  globalThis.fetch = async () =>
    new Response("archive", { headers: { "Content-Type": "application/zip" } });
  expect(
    await (
      await api<Blob>("/account/export", { body: {}, response: "blob" })
    ).text(),
  ).toBe("archive");
});

test("errori API: status e soli messaggi di campo testuali", async () => {
  globalThis.fetch = async () =>
    Response.json(
      {
        error: {
          message: "Controlla i campi.",
          fields: {
            email: "Email non valida.",
            password: { nested: "invalid" },
          },
        },
      },
      { status: 422 },
    );
  await expect(api("/auth/register", { body: {} })).rejects.toMatchObject({
    status: 422,
    message: "Controlla i campi.",
    fields: { email: "Email non valida." },
  });
});

test("HTML e JSON di errore inatteso non espongono contenuto tecnico", async () => {
  for (const body of [
    "<html>private proxy details</html>",
    "null",
    '{"error":null}',
    '{"error":{"message":{},"fields":null}}',
  ]) {
    globalThis.fetch = async () => new Response(body, { status: 503 });
    await expect(api("/feed/home")).rejects.toMatchObject({
      status: 503,
      message: "Servizio non disponibile. Riprova tra poco.",
    });
  }
});

test("JSON di successo corrotto diventa un errore comprensibile", async () => {
  globalThis.fetch = async () => new Response("<html>private response</html>");
  await expect(api("/feed/home")).rejects.toMatchObject({
    status: 0,
    message:
      "Il servizio ha restituito una risposta non valida. Riprova tra poco.",
  });
  await expect(api("/posts", { body: {} })).rejects.toThrow(
    "L’operazione potrebbe essere riuscita",
  );
});

test("rete interrotta: messaggio diverso per letture e scritture, nessun retry", async () => {
  let calls = 0;
  globalThis.fetch = async () => {
    calls++;
    throw new TypeError("private socket details");
  };
  await expect(api("/feed/home")).rejects.toThrow("Controlla la rete");
  await expect(api("/posts", { body: { body: "Draft" } })).rejects.toThrow(
    "L’operazione potrebbe essere riuscita",
  );
  expect(calls).toBe(2);
});

test("timeout di lettura cancella la richiesta", async () => {
  let signal: AbortSignal | undefined;
  globalThis.fetch = async (_, init) => {
    signal = init?.signal ?? undefined;
    return new Promise((_, reject) =>
      signal!.addEventListener(
        "abort",
        () => reject(new DOMException("Aborted", "AbortError")),
        { once: true },
      ),
    );
  };
  await expect(api("/feed/home", { timeoutMs: 10 })).rejects.toThrow(
    "troppo tempo",
  );
  expect(signal?.aborted).toBe(true);
});

test("timeout copre anche il corpo della risposta e non ripete la scrittura", async () => {
  let calls = 0;
  globalThis.fetch = async (_, init) => {
    calls++;
    return new Response(
      new ReadableStream({
        start(controller) {
          controller.enqueue(new TextEncoder().encode('{"data":'));
          init!.signal!.addEventListener(
            "abort",
            () => controller.error(new DOMException("Aborted", "AbortError")),
            { once: true },
          );
        },
      }),
    );
  };
  await expect(api("/posts", { body: {}, timeoutMs: 10 })).rejects.toThrow(
    "L’operazione potrebbe essere riuscita",
  );
  expect(calls).toBe(1);
});

test("cancellazione del chiamante preserva AbortError", async () => {
  globalThis.fetch = async (_, init) =>
    new Promise((_, reject) =>
      init!.signal!.addEventListener(
        "abort",
        () => reject(new DOMException("Aborted", "AbortError")),
        { once: true },
      ),
    );
  const controller = new AbortController();
  const pending = api("/feed/home", { signal: controller.signal });
  controller.abort();
  await expect(pending).rejects.toMatchObject({ name: "AbortError" });
  await expect(pending).rejects.not.toBeInstanceOf(ApiError);
});
