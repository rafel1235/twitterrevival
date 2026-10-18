"use client";
import { useRouter } from "next/navigation";
import { useState } from "react";
import { SessionGate } from "./shared";
import styles from "./profile.module.css";
export function People() {
  const router = useRouter();
  const [handle, setHandle] = useState("");
  return (
    <SessionGate>
      <header className="feed-header">
        <div>
          <h1>Persone</h1>
          <p>Le connessioni cominciano con un nome</p>
        </div>
      </header>
      <section className={styles.content}>
        <h2>Conosci già qualcuno?</h2>
        <p>
          Inserisci l’handle esatto per aprire il suo profilo. La ricerca per
          parole chiave arriverà nello Sprint 4.
        </p>
        <form
          aria-label="Apri un profilo"
          onSubmit={(event) => {
            event.preventDefault();
            router.push(
              `/u/${encodeURIComponent(handle.replace(/^@/, "").toLowerCase())}`,
            );
          }}
        >
          <div className="form-field">
            <label htmlFor="person-handle">Handle della persona</label>
            <input
              id="person-handle"
              value={handle}
              onChange={(e) => setHandle(e.target.value)}
              pattern="@?[A-Za-z0-9_]{3,20}"
              required
              placeholder="nome_utente"
              autoCapitalize="none"
              spellCheck={false}
            />
          </div>
          <button type="submit" className="button primary submit-button">
            Apri profilo
          </button>
        </form>
      </section>
    </SessionGate>
  );
}
