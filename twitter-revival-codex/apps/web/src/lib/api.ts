export type User = {
  id: string;
  handle: string;
  email: string;
  display_name: string;
  created_at: string;
  email_verified: boolean;
  avatar_style: string;
};
type Failure = {
  error?: { message?: string; fields?: Record<string, string> };
};

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
    signal?: AbortSignal;
    method?: "GET" | "POST" | "PATCH" | "DELETE";
  } = {},
): Promise<T> {
  let response: Response;
  const method =
    options.method ?? (options.body === undefined ? "GET" : "POST");
  const mutation = method !== "GET";
  try {
    response = await fetch(`/api/v1${path}`, {
      method,
      credentials: "same-origin",
      cache: "no-store",
      signal: options.signal,
      headers: !mutation
        ? undefined
        : { "Content-Type": "application/json", "X-CSRF-Protection": "1" },
      body: mutation ? JSON.stringify(options.body ?? {}) : undefined,
    });
  } catch (error) {
    if (error instanceof Error && error.name === "AbortError") throw error;
    throw new ApiError(
      0,
      "Connessione al servizio non riuscita. Controlla la rete e riprova.",
    );
  }
  if (!response.ok) {
    if (
      response.status === 401 &&
      path !== "/auth/login" &&
      typeof window !== "undefined"
    )
      window.dispatchEvent(new Event("radura:session-expired"));
    const payload: Failure = await response.json().catch(() => ({}));
    throw new ApiError(
      response.status,
      payload.error?.message ?? "Servizio non disponibile. Riprova tra poco.",
      payload.error?.fields,
    );
  }
  if (response.status === 204) return undefined as T;
  return response.json() as Promise<T>;
}
