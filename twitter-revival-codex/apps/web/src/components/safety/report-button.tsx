"use client";
import Link from "next/link";
import { useState } from "react";
import { api, ApiError } from "@/lib/api";
import styles from "./safety.module.css";

export function ReportButton({
  type,
  id,
}: {
  type: "post" | "user";
  id: string;
}) {
  const [pending, setPending] = useState(false);
  const [sent, setSent] = useState(false);
  const [error, setError] = useState("");
  return (
    <details className={styles.disclosure}>
      <summary>Segnala {type === "post" ? "post" : "profilo"}</summary>
      {sent ? (
        <p role="status">
          Segnalazione ricevuta.{" "}
          <Link href="/settings/reports">Segui l’esito</Link>.
        </p>
      ) : (
        <form
          aria-label="Invia segnalazione"
          className={styles.form}
          onSubmit={async (event) => {
            event.preventDefault();
            const form = new FormData(event.currentTarget);
            setPending(true);
            setError("");
            try {
              await api("/reports", {
                body: {
                  target_type: type,
                  target_id: id,
                  reason: form.get("reason"),
                  details: form.get("details"),
                },
              });
              setSent(true);
            } catch (err) {
              setError(
                err instanceof ApiError
                  ? Object.values(err.fields).join(" ") || err.message
                  : "Invio non riuscito.",
              );
            } finally {
              setPending(false);
            }
          }}
        >
          <p>
            Spiega cosa è successo. Un moderatore esaminerà la segnalazione;
            evita dati sensibili non necessari.
          </p>
          <label>
            Motivo
            <select name="reason" aria-label="Motivo" disabled={pending}>
              <option value="spam">Spam</option>
              <option value="harassment">Molestie o abusi</option>
              <option value="illegal">Contenuto ritenuto illecito</option>
              <option value="other">Altro</option>
            </select>
          </label>
          <label>
            Dettagli
            <textarea
              name="details"
              required
              minLength={10}
              maxLength={2000}
              rows={4}
              disabled={pending}
            />
          </label>
          <button className="button primary" disabled={pending}>
            {pending ? "Invio…" : "Invia segnalazione"}
          </button>
          {error && (
            <p role="alert" className="form-error">
              {error}
            </p>
          )}
        </form>
      )}
    </details>
  );
}
