"use client";
import Link from "next/link";
import { useCallback, useEffect, useRef, useState } from "react";
import { api } from "@/lib/api";
import type { PostPage } from "@/lib/posts";
import { PageError } from "@/components/profile/shared";
import { PostCard } from "./post-card";
import { Icon } from "@/components/layout/icon";
import styles from "./posts.module.css";

export function PostList({
  path,
  empty,
  label = "Post",
}: {
  path: string;
  empty: string;
  label?: string;
}) {
  const [data, setData] = useState<PostPage>({ items: [] });
  const [pending, setPending] = useState(true);
  const [error, setError] = useState("");
  const sequence = useRef(0);
  const lifetime = useRef<AbortController | null>(null);
  const load = useCallback(
    (cursor = "", signal = lifetime.current?.signal) => {
      const request = ++sequence.current;
      return api<{ data: PostPage }>(
        `${path}?limit=20${cursor ? `&cursor=${encodeURIComponent(cursor)}` : ""}`,
        { signal },
      )
        .then((result) => {
          if (signal?.aborted || request !== sequence.current) return;
          setData((previous) => ({
            ...result.data,
            items: cursor
              ? [
                  ...previous.items,
                  ...result.data.items.filter(
                    (item) => !previous.items.some((old) => old.id === item.id),
                  ),
                ]
              : result.data.items,
          }));
          setError("");
        })
        .catch((err: unknown) => {
          if (signal?.aborted || request !== sequence.current) return;
          if (!cursor) setData({ items: [] });
          setError(
            err instanceof Error ? err.message : "Caricamento non riuscito.",
          );
        })
        .finally(() => {
          if (!signal?.aborted && request === sequence.current)
            setPending(false);
        });
    },
    [path],
  );
  useEffect(() => {
    const controller = new AbortController();
    lifetime.current = controller;
    void load("", controller.signal);
    const refresh = () => {
      setPending(true);
      void load("", controller.signal);
    };
    window.addEventListener("radura:posts-changed", refresh);
    return () => {
      controller.abort();
      window.removeEventListener("radura:posts-changed", refresh);
    };
  }, [load]);
  return (
    <section aria-label={label} aria-busy={pending}>
      <div className={styles.listHeading}>
        <span className="muted">Dal più recente</span>
        <button
          className="text-button"
          disabled={pending}
          onClick={() => {
            setPending(true);
            void load();
          }}
        >
          Aggiorna
        </button>
      </div>
      {error && (
        <PageError
          message={error}
          retry={() => {
            setPending(true);
            void load();
          }}
        />
      )}
      {pending && (
        <p className={styles.state} role="status">
          Caricamento post…
        </p>
      )}
      {!pending && !error && data.items.length === 0 && (
        <p className={styles.state}>{empty}</p>
      )}
      {data.items.map((event) => (
        <div key={event.id}>
          {event.reposted_by && (
            <p className={styles.reposted}>
              <Icon name="repeat" size={14} />
              <Link href={`/u/${event.reposted_by.handle}`}>
                Ripubblicato da @{event.reposted_by.handle}
              </Link>
            </p>
          )}
          <PostCard post={event.post} />
        </div>
      ))}
      {data.next_cursor && (
        <div className={styles.state}>
          <button
            className="button primary"
            disabled={pending}
            onClick={() => {
              setPending(true);
              void load(data.next_cursor);
            }}
          >
            Carica altri post
          </button>
        </div>
      )}
    </section>
  );
}
