"use client";
import { useCallback, useEffect, useRef, useState } from "react";
import { api, ApiError } from "./api";

export function useResource<T>(path: string) {
  const [data, setData] = useState<T | null>(null);
  const [error, setError] = useState("");
  const [status, setStatus] = useState(0);
  const [loading, setLoading] = useState(true);
  const [owner, setOwner] = useState(path);
  const sequence = useRef(0);
  const active = useRef<AbortController | null>(null);
  const dataPath = useRef(path);
  const load = useCallback(() => {
    active.current?.abort();
    const controller = new AbortController();
    active.current = controller;
    const request = ++sequence.current;
    const current = () =>
      !controller.signal.aborted && request === sequence.current;
    return api<T>(path, { signal: controller.signal })
      .then((result) => {
        if (!current()) return;
        setOwner(path);
        dataPath.current = path;
        setData(result);
        setError("");
        setStatus(200);
      })
      .catch((err: unknown) => {
        if (!current()) return;
        setOwner(path);
        if (
          dataPath.current !== path ||
          (err instanceof ApiError && [401, 403, 404].includes(err.status))
        )
          setData(null);
        dataPath.current = path;
        setError(
          err instanceof Error ? err.message : "Richiesta non riuscita.",
        );
        setStatus(err instanceof ApiError ? err.status : 0);
      })
      .finally(() => {
        if (current()) setLoading(false);
      });
  }, [path]);
  useEffect(() => {
    void load();
    return () => {
      active.current?.abort();
    };
  }, [load]);
  const refresh = useCallback(() => {
    setLoading(true);
    return load();
  }, [load]);
  return owner === path
    ? { data, error, status, loading, refresh }
    : { data: null, error: "", status: 0, loading: true, refresh };
}
