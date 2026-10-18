export type User = {
  id: string;
  handle: string;
  email: string;
  display_name: string;
  created_at: string;
  email_verified: boolean;
  avatar_style: string;
  role: "member" | "moderator";
};
type Failure = {
  error?: { message?: string; fields?: Record<string, string> };
};

function failure(value: unknown): Failure {
  if (!value || typeof value !== "object" || !("error" in value)) return {};
  const error = value.error;
  if (!error || typeof error !== "object") return {};
  const message =
    "message" in error && typeof error.message === "string"
      ? error.message
      : undefined;
  const fields =
    "fields" in error && error.fields && typeof error.fields === "object"
      ? Object.fromEntries(
          Object.entries(error.fields).filter(
            ([, text]) => typeof text === "string",
          ),
        )
      : undefined;
  return { error: { message, fields } };
}

export class ApiError extends Error {
  constructor(
    public status: number,
    message: string,
    public fields: Record<string, string> = {},
  ) {
    super(message);
  }
}

export async function api<T>(
  path: string,
  options: {
    body?: unknown;
    form?: FormData;
    signal?: AbortSignal;
    method?: "GET" | "POST" | "PATCH" | "DELETE";
    response?: "blob";
    timeoutMs?: number;
  } = {},
): Promise<T> {
  const method =
    options.method ??
    (options.body === undefined && !options.form ? "GET" : "POST");
  const mutation = method !== "GET";
  const controller = new AbortController();
  const cancel = () => controller.abort(options.signal?.reason);
  if (options.signal?.aborted) cancel();
  else options.signal?.addEventListener("abort", cancel, { once: true });
  let timedOut = false;
  const timeout = setTimeout(
    () => {
      timedOut = true;
      controller.abort();
    },
    options.timeoutMs ??
      (options.response === "blob" ? 60000 : mutation ? 30000 : 15000),
  );
  try {
    const response = await fetch(`/api/v1${path}`, {
      method,
      credentials: "same-origin",
      cache: "no-store",
      signal: controller.signal,
      headers: !mutation
        ? undefined
        : options.form
          ? { "X-CSRF-Protection": "1" }
          : { "Content-Type": "application/json", "X-CSRF-Protection": "1" },
      body: mutation
        ? (options.form ?? JSON.stringify(options.body ?? {}))
        : undefined,
    });
    if (!response.ok) {
      if (
        response.status === 401 &&
        path !== "/auth/login" &&
        typeof window !== "undefined"
      )
        window.dispatchEvent(new Event("radura:session-expired"));
      let payload: Failure = {};
      try {
        payload = failure(await response.json());
      } catch (error) {
        if (controller.signal.aborted) throw error;
      }
      throw new ApiError(
        response.status,
        payload.error?.message ?? "Servizio non disponibile. Riprova tra poco.",
        payload.error?.fields,
      );
    }
    if (response.status === 204) return undefined as T;
    if (options.response === "blob") return (await response.blob()) as T;
    try {
      return (await response.json()) as T;
    } catch (error) {
      if (controller.signal.aborted) throw error;
      throw new ApiError(
        0,
        mutation
          ? "Il servizio ha restituito una risposta non valida. L’operazione potrebbe essere riuscita: verifica prima di riprovare."
          : "Il servizio ha restituito una risposta non valida. Riprova tra poco.",
      );
    }
  } catch (error) {
    if (options.signal?.aborted) throw error;
    if (timedOut)
      throw new ApiError(
        0,
        mutation
          ? "Tempo di attesa esaurito. L’operazione potrebbe essere riuscita: verifica prima di riprovare."
          : "Il servizio impiega troppo tempo a rispondere. Riprova tra poco.",
      );
    if (error instanceof ApiError) throw error;
    throw new ApiError(
      0,
      mutation
        ? "Connessione interrotta. L’operazione potrebbe essere riuscita: verifica prima di riprovare."
        : "Connessione al servizio non riuscita. Controlla la rete e riprova.",
    );
  } finally {
    clearTimeout(timeout);
    options.signal?.removeEventListener("abort", cancel);
  }
}
