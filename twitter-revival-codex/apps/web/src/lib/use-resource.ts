"use client";
import { useCallback, useEffect, useState } from "react";
import { api, ApiError } from "./api";

export function useResource<T>(path: string) {
  const [data, setData] = useState<T | null>(null);
  const [error, setError] = useState("");
  const [status, setStatus] = useState(0);
  const [loading, setLoading] = useState(true);
  const load = useCallback(
    (signal?: AbortSignal) =>
      api<T>(path, { signal })
        .then((result) => {
          setData(result);
          setError("");
          setStatus(200);
        })
        .catch((err: unknown) => {
          if (signal?.aborted) return;
          setError(
            err instanceof Error ? err.message : "Richiesta non riuscita.",
          );
          setStatus(err instanceof ApiError ? err.status : 0);
        })
        .finally(() => {
          if (!signal?.aborted) setLoading(false);
        }),
    [path],
  );
  useEffect(() => {
    const controller = new AbortController();
    void load(controller.signal);
    return () => controller.abort();
  }, [load]);
  const refresh = useCallback(() => {
    setLoading(true);
    return load();
  }, [load]);
  return { data, error, status, loading, refresh };
}
