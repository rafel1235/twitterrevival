"use client";
import Link from "next/link";
import { useSession } from "@/components/auth/session-provider";
import styles from "./profile.module.css";

export function SessionGate({ children }: { children: React.ReactNode }) {
  const { user, loading, error, refresh } = useSession();
  if (loading && !user)
    return (
      <p className={styles.state} role="status">
        Verifica sessione…
      </p>
    );
  if (!user)
    return (
      <section className={styles.state}>
        <h1>{error ? "Servizio non disponibile" : "Entra nella community"}</h1>
        <p>
          {error || "Accedi per vedere i profili e creare le tue connessioni."}
        </p>
        {error ? (
          <button className="text-button" onClick={() => void refresh()}>
            Riprova
          </button>
        ) : (
          <Link href="/login" className="button primary">
            Accedi
          </Link>
        )}
      </section>
    );
  return children;
}
export function UserAvatar({
  name,
  style = "sage",
  large = false,
  small = false,
}: {
  name: string;
  style?: string;
  large?: boolean;
  small?: boolean;
}) {
  const initials = name
    .trim()
    .split(/\s+/)
    .slice(0, 2)
    .map((word) => Array.from(word)[0] || "")
    .join("")
    .toUpperCase();
  const safe = ["sage", "peach", "lavender", "sky"].includes(style)
    ? style
    : "sage";
  return (
    <span
      className={`avatar ${safe} ${large ? styles.largeAvatar : ""} ${small ? "small" : ""}`}
      aria-hidden="true"
    >
      {initials}
    </span>
  );
}
export function PageError({
  message,
  retry,
}: {
  message: string;
  retry?: () => void;
}) {
  return (
    <div className={styles.state} role="alert">
      <p>{message}</p>
      {retry && (
        <button className="text-button" onClick={retry}>
          Riprova
        </button>
      )}
    </div>
  );
}
export function LocalMailbox() {
  const link = process.env.NEXT_PUBLIC_DEV_MAILBOX_URL;
  if (!link) return null;
  let valid = false;
  try {
    const parsed = new URL(link);
    valid =
      ["localhost", "127.0.0.1"].includes(parsed.hostname) &&
      ["http:", "https:"].includes(parsed.protocol);
  } catch {}
  return valid ? (
    <p className={styles.localMail}>
      Ambiente locale: le email arrivano nella{" "}
      <a href={link} target="_blank" rel="noreferrer noopener">
        casella di prova
      </a>
      , senza invio su Internet.
    </p>
  ) : null;
}
