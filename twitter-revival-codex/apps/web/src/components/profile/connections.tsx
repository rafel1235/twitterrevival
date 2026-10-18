"use client";
import Link from "next/link";
import { useCallback, useEffect, useRef, useState } from "react";
import { api, ApiError } from "@/lib/api";
import type { ConnectionPage } from "@/lib/profile";
import { PageError, SessionGate, UserAvatar } from "./shared";
import styles from "./profile.module.css";

export function ConnectionList({
  path,
  empty,
  blocked = false,
  label,
}: {
  path: string;
  empty: string;
  blocked?: boolean;
  label?: string;
}) {
  const [page, setPage] = useState<ConnectionPage>({ users: [] });
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [pending, setPending] = useState("");
  const sequence = useRef(0);
  const lifetime = useRef<AbortController | null>(null);
  const lastCursor = useRef<string | undefined>(undefined);
  const load = useCallback(
    (cursor?: string, signal = lifetime.current?.signal) => {
      const request = ++sequence.current;
      lastCursor.current = cursor;
      return api<{ data: ConnectionPage }>(
        `${path}${path.includes("?") ? "&" : "?"}limit=20${cursor ? `&cursor=${encodeURIComponent(cursor)}` : ""}`,
        { signal },
      )
        .then((response) => {
          if (signal?.aborted || request !== sequence.current) return;
          setPage((previous) =>
            cursor
              ? {
                  ...response.data,
                  users: [
                    ...previous.users,
                    ...response.data.users.filter(
                      (person) =>
                        !previous.users.some((old) => old.id === person.id),
                    ),
                  ],
                }
              : response.data,
          );
          setError("");
        })
        .catch((err: unknown) => {
          if (signal?.aborted || request !== sequence.current) return;
          if (
            !signal?.aborted &&
            err instanceof ApiError &&
            [401, 403, 404].includes(err.status)
          )
            setPage({ users: [] });
          if (!signal?.aborted)
            setError(
              err instanceof Error ? err.message : "Caricamento non riuscito.",
            );
        })
        .finally(() => {
          if (!signal?.aborted && request === sequence.current)
            setLoading(false);
        });
    },
    [path],
  );
  useEffect(() => {
    const controller = new AbortController();
    lifetime.current = controller;
    void load(undefined, controller.signal);
    return () => controller.abort();
  }, [load]);
  async function unblock(id: string) {
    setPending(id);
    setError("");
    try {
      await api(`/users/${id}/block`, { method: "DELETE", body: {} });
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Sblocco non riuscito.");
    } finally {
      setPending("");
    }
  }
  return (
    <section
      className={styles.connectionList}
      aria-label={
        label || (blocked ? "Elenco utenti bloccati" : "Elenco connessioni")
      }
      aria-busy={loading || !!pending}
    >
      {error && (
        <PageError
          message={error}
          pending={loading || !!pending}
          stale={page.users.length > 0}
          retry={() => {
            setLoading(true);
            void load(lastCursor.current);
          }}
        />
      )}
      <div aria-live="polite">
        {page.users.map((person) => (
          <div className={styles.personRow} key={person.id}>
            <UserAvatar
              name={person.display_name}
              style={person.avatar_style}
            />
            <div className={styles.personInfo}>
              {blocked ? (
                <strong>{person.display_name}</strong>
              ) : (
                <Link href={`/u/${person.handle}`}>
                  <strong>{person.display_name}</strong>
                </Link>
              )}
              <span className="muted">@{person.handle}</span>
            </div>
            {blocked && (
              <button
                className="button"
                disabled={!!pending}
                aria-label={`Sblocca @${person.handle}`}
                onClick={() => void unblock(person.id)}
              >
                {pending === person.id ? "Sblocco…" : "Sblocca"}
              </button>
            )}
          </div>
        ))}
      </div>
      {loading ? (
        <p className={styles.state} role="status">
          Caricamento…
        </p>
      ) : page.users.length === 0 && !error ? (
        <p className={styles.state}>{empty}</p>
      ) : null}
      {page.next_cursor && (
        <div className={styles.content}>
          <button
            className="text-button"
            disabled={loading}
            onClick={() => {
              setLoading(true);
              void load(page.next_cursor);
            }}
          >
            Carica altri
          </button>
        </div>
      )}
    </section>
  );
}
export function BlockedUsers() {
  return (
    <SessionGate>
      <header className="feed-header">
        <div>
          <h1>Utenti bloccati</h1>
          <p>Solo tu puoi vedere questo elenco</p>
        </div>
        <Link href="/settings/profile" className="text-button">
          Impostazioni
        </Link>
      </header>
      <p className={styles.content}>
        Il blocco nasconde i profili in entrambe le direzioni e rimuove i follow
        reciproci.
      </p>
      <ConnectionList
        path="/users/me/blocks"
        empty="Non hai bloccato nessuno."
        blocked
      />
    </SessionGate>
  );
}
