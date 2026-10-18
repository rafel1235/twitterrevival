"use client";
import Link from "next/link";
import { useCallback, useEffect, useRef, useState } from "react";
import { api } from "@/lib/api";
import {
  notificationsChanged,
  type NotificationPage,
} from "@/lib/notifications";
import { useSession } from "@/components/auth/session-provider";
import {
  SessionGate,
  PageError,
  UserAvatar,
} from "@/components/profile/shared";
import styles from "./discovery.module.css";
const labels = {
  follow: "ha iniziato a seguirti",
  like: "ha messo mi piace al tuo post",
  repost: "ha ripubblicato il tuo post",
  reply: "ha risposto al tuo post",
  quote: "ha citato il tuo post",
};
export function NotificationsView() {
  const { user } = useSession();
  return (
    <SessionGate>
      <NotificationContent key={user?.id} />
    </SessionGate>
  );
}
function NotificationContent() {
  const [page, setPage] = useState<NotificationPage>({
    items: [],
    unread_count: 0,
  });
  const [pending, setPending] = useState(true);
  const [error, setError] = useState("");
  const [writing, setWriting] = useState("");
  const lifetime = useRef<AbortController | null>(null);
  const sequence = useRef(0);
  const load = useCallback((cursor = "", signal = lifetime.current?.signal) => {
    const current = ++sequence.current;
    return api<{ data: NotificationPage }>(
      `/notifications?limit=20${cursor ? `&cursor=${encodeURIComponent(cursor)}` : ""}`,
      { signal },
    )
      .then((response) => {
        if (signal?.aborted || current !== sequence.current) return;
        setPage((previous) => ({
          ...response.data,
          items: cursor
            ? [
                ...previous.items,
                ...response.data.items.filter(
                  (item) => !previous.items.some((old) => old.id === item.id),
                ),
              ]
            : response.data.items,
        }));
        setError("");
      })
      .catch((err: unknown) => {
        if (signal?.aborted || current !== sequence.current) return;
        if (!cursor) setPage({ items: [], unread_count: 0 });
        setError(
          err instanceof Error ? err.message : "Caricamento non riuscito.",
        );
      })
      .finally(() => {
        if (!signal?.aborted && current === sequence.current) setPending(false);
      });
  }, []);
  useEffect(() => {
    const controller = new AbortController();
    lifetime.current = controller;
    void load("", controller.signal);
    const refresh = () => {
      if (document.visibilityState === "visible") {
        setPending(true);
        void load("", controller.signal);
      }
    };
    const timer = setInterval(refresh, 30000);
    window.addEventListener("radura:notifications-changed", refresh);
    document.addEventListener("visibilitychange", refresh);
    return () => {
      controller.abort();
      clearInterval(timer);
      window.removeEventListener("radura:notifications-changed", refresh);
      document.removeEventListener("visibilitychange", refresh);
    };
  }, [load]);
  async function read(id = "all") {
    setWriting(id);
    setError("");
    try {
      await api(
        id === "all" ? "/notifications/read" : `/notifications/${id}/read`,
        { body: {} },
      );
      notificationsChanged();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Operazione non riuscita.");
    } finally {
      setWriting("");
    }
  }
  return (
    <>
      <header className="feed-header">
        <div>
          <h1>Notifiche</h1>
          <p>{page.unread_count} da leggere · Aggiornamento ogni 30 secondi</p>
        </div>
      </header>
      <div className={styles.toolbar}>
        <button
          className="text-button"
          disabled={pending || !!writing}
          onClick={() => {
            setPending(true);
            void load();
          }}
        >
          Aggiorna notifiche
        </button>
        <button
          className="text-button"
          disabled={pending || !!writing || page.unread_count === 0}
          onClick={() => void read()}
        >
          Segna tutte come lette
        </button>
      </div>
      {error && <PageError message={error} retry={() => void load()} />}
      {pending && (
        <p className={styles.state} role="status">
          Caricamento notifiche…
        </p>
      )}
      {!pending && !error && page.items.length === 0 && (
        <div className={styles.state}>
          <h2>Qui arriveranno le novità.</h2>
          <p>
            Follow, mi piace, repost, risposte e citazioni. Nessuna notifica per
            le tue azioni verso te stesso.
          </p>
        </div>
      )}
      <section aria-label="Elenco notifiche" aria-busy={pending}>
        {page.items.map((item) => (
          <article
            key={item.id}
            className={`${styles.notification} ${!item.read_at ? styles.unread : ""}`}
            aria-label={`Notifica: @${item.actor.handle} ${labels[item.type]}`}
            data-testid={`notification-${item.id}`}
          >
            <Link
              href={`/u/${item.actor.handle}`}
              aria-label={`Profilo di @${item.actor.handle}`}
            >
              <UserAvatar
                name={item.actor.display_name}
                style={item.actor.avatar_style}
              />
            </Link>
            <div>
              <p>
                <Link href={`/u/${item.actor.handle}`}>
                  <strong>{item.actor.display_name}</strong>
                </Link>{" "}
                {labels[item.type]}
              </p>
              <time className="muted" dateTime={item.created_at}>
                {new Date(item.created_at).toLocaleString("it-IT", {
                  dateStyle: "medium",
                  timeStyle: "short",
                })}
              </time>
              <div className={styles.rowActions}>
                {item.post_id ? (
                  <Link className="text-button" href={`/posts/${item.post_id}`}>
                    Apri conversazione
                  </Link>
                ) : (
                  <Link
                    className="text-button"
                    href={`/u/${item.actor.handle}`}
                  >
                    Apri profilo
                  </Link>
                )}
                {!item.read_at && (
                  <button
                    className="text-button"
                    disabled={!!writing}
                    onClick={() => void read(item.id)}
                  >
                    Segna come letta
                  </button>
                )}
                {item.read_at && <span className="muted">Letta</span>}
              </div>
            </div>
          </article>
        ))}
      </section>
      {page.next_cursor && (
        <div className={styles.state}>
          <button
            className="button primary"
            disabled={pending}
            onClick={() => {
              setPending(true);
              void load(page.next_cursor);
            }}
          >
            Carica altre notifiche
          </button>
        </div>
      )}
    </>
  );
}
