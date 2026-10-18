"use client";
import Link from "next/link";
import { useState } from "react";
import { api, ApiError } from "@/lib/api";
import { useSession } from "@/components/auth/session-provider";
import { SessionGate } from "@/components/profile/shared";
import { SettingsNav } from "./settings-nav";
import styles from "./safety.module.css";

export function AccountSettings() {
  const { setUser } = useSession();
  const [password, setPassword] = useState("");
  const [confirmation, setConfirmation] = useState("");
  const [pending, setPending] = useState("");
  const [message, setMessage] = useState("");
  const [error, setError] = useState("");
  const [deleted, setDeleted] = useState(false);
  async function act(action: "export" | "delete") {
    setPending(action);
    setMessage("");
    setError("");
    try {
      if (action === "export") {
        const blob = await api<Blob>("/account/export", {
          body: { password },
          response: "blob",
        });
        const url = URL.createObjectURL(blob);
        const link = document.createElement("a");
        link.href = url;
        link.download = "radura-export.zip";
        document.body.append(link);
        link.click();
        link.remove();
        setTimeout(() => URL.revokeObjectURL(url), 60000);
        setMessage(
          "Archivio pronto: contiene data.json e le tue immagini. Conservalo in un luogo privato.",
        );
      } else {
        await api("/account", {
          method: "DELETE",
          body: { password, confirmation },
        });
        setDeleted(true);
        setUser(null);
      }
      setPassword("");
    } catch (err) {
      setError(
        err instanceof ApiError
          ? Object.values(err.fields).join(" ") || err.message
          : "Operazione non riuscita. Riprova.",
      );
    } finally {
      setPending("");
    }
  }
  if (deleted)
    return (
      <section className={styles.content} role="status">
        <h1>Account eliminato</h1>
        <p>
          Sessioni revocate e dati del profilo rimossi. Le immagini sono in coda
          di eliminazione; eventuali copie di backup scadono secondo la politica
          di conservazione.
        </p>
        <Link className="text-button" href="/">
          Torna alla home
        </Link>
      </section>
    );
  return (
    <SessionGate>
      <header className="feed-header">
        <div>
          <h1>I tuoi dati</h1>
          <p>Esporta una copia o elimina il tuo account</p>
        </div>
      </header>
      <SettingsNav />
      <div className={styles.content}>
        <p>
          Entrambe le operazioni richiedono la password attuale. L’archivio
          include i dati che hai fornito e le immagini elaborate dal servizio.
        </p>
        <form
          className={styles.form}
          onSubmit={(e) => {
            e.preventDefault();
            void act("export");
          }}
        >
          <label>
            Password attuale
            <input
              type="password"
              autoComplete="current-password"
              required
              maxLength={128}
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              disabled={!!pending}
            />
          </label>
          <button className="button primary" disabled={!!pending || !password}>
            {pending === "export"
              ? "Preparazione archivio…"
              : "Scarica i tuoi dati"}
          </button>
        </form>
        <section className={styles.danger} aria-label="Eliminazione account">
          <h2>Elimina account</h2>
          <p>
            Questa operazione è definitiva. Cancella profilo, testo dei tuoi
            post, connessioni e sessioni. Le risposte scritte da altre persone
            restano prive del tuo contenuto originale. Le immagini vengono
            rimosse dallo storage con tentativi automatici in caso di errore.
          </p>
          <p>
            Rimuove anche le tue iscrizioni e archivia le community che
            possiedi, cancellandone nome, descrizione e regole. Gli altri membri
            perderanno l’accesso alle conversazioni; i loro post non diventano
            pubblici.
          </p>
          <p>
            Un audit limitato e pseudonimo può restare per 180 giorni; i backup
            locali richiedono una gestione separata.{" "}
            <Link href="/privacy">Leggi le informazioni sui dati</Link>.
          </p>
          <form
            className={styles.form}
            onSubmit={(e) => {
              e.preventDefault();
              void act("delete");
            }}
          >
            <label>
              Scrivi ELIMINA
              <input
                value={confirmation}
                onChange={(e) => setConfirmation(e.target.value)}
                autoComplete="off"
                disabled={!!pending}
                required
                pattern="ELIMINA"
              />
            </label>
            <button
              className="button"
              disabled={!!pending || !password || confirmation !== "ELIMINA"}
            >
              {pending === "delete"
                ? "Eliminazione…"
                : "Elimina definitivamente il mio account"}
            </button>
          </form>
        </section>
        {message && (
          <p className="form-success" role="status">
            {message}
          </p>
        )}
        {error && (
          <p className="form-error" role="alert">
            {error}
          </p>
        )}
      </div>
    </SessionGate>
  );
}
