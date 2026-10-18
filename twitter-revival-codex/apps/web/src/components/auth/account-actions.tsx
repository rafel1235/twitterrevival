"use client";
import Link from "next/link";
import { useSearchParams } from "next/navigation";
import { useState } from "react";
import { api, ApiError } from "@/lib/api";
import { useSession } from "./session-provider";
import { Icon } from "@/components/layout/icon";
import { LocalMailbox } from "@/components/profile/shared";
import { useFormFocus } from "@/lib/use-form-focus";

export function AccountAction({
  mode,
}: {
  mode: "forgot" | "reset" | "verify";
}) {
  const search = useSearchParams();
  const token = search.get("token") || "";
  const { setUser, refresh } = useSession();
  const [pending, setPending] = useState(false);
  const [message, setMessage] = useState("");
  const [error, setError] = useState("");
  const [fields, setFields] = useState<Record<string, string>>({});
  const formRef = useFormFocus(pending, fields, error);
  const title =
    mode === "forgot"
      ? "Ritrova il tuo account"
      : mode === "reset"
        ? "Una nuova password"
        : "Conferma la tua email";
  async function submit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setMessage("");
    setError("");
    setFields({});
    const form = new FormData(event.currentTarget);
    const password = String(form.get("password") || "");
    if (mode === "reset") {
      if (
        Array.from(password).length < 12 ||
        new TextEncoder().encode(password).length > 128
      ) {
        setFields({
          password: "Usa almeno 12 caratteri e al massimo 128 byte.",
        });
        return;
      }
      if (password !== form.get("confirmation")) {
        setFields({ confirmation: "Le password non coincidono." });
        return;
      }
    }
    setPending(true);
    try {
      const path =
        mode === "forgot"
          ? "/auth/password/forgot"
          : mode === "reset"
            ? "/auth/password/reset"
            : "/auth/verification/confirm";
      const body =
        mode === "forgot"
          ? { email: String(form.get("email")) }
          : mode === "reset"
            ? { token, password }
            : { token };
      const result = await api<{ data: { message: string } }>(path, { body });
      setMessage(result.data.message);
      if (mode === "reset") {
        setUser(null);
        window.history.replaceState(null, "", "/reset-password");
      }
      if (mode === "verify") {
        await refresh();
        window.history.replaceState(null, "", "/verify-email");
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : "Richiesta non riuscita.");
      if (err instanceof ApiError) setFields(err.fields);
    } finally {
      setPending(false);
    }
  }
  return (
    <>
      <header className="feed-header">
        <div>
          <h1>{mode === "verify" ? "Verifica email" : "Sicurezza account"}</h1>
          <p>Un passo per proteggere il tuo spazio</p>
        </div>
        <Icon name="leaf" />
      </header>
      <section className="auth-card">
        <span className="auth-mark">
          <Icon name="leaf" size={36} />
        </span>
        <h2>{title}</h2>
        <p className="auth-intro">
          {mode === "forgot"
            ? "Inserisci l’email usata per registrarti. Ti invieremo un link se è associata a un account attivo."
            : mode === "reset"
              ? "Scegli una password unica. Il cambio chiuderà tutte le sessioni aperte."
              : "Premi il pulsante per confermare che questo indirizzo email appartiene a te."}
        </p>
        {message ? (
          <div role="status" className="form-success">
            <p>{message}</p>
            <Link
              className="button primary submit-button"
              href={mode === "verify" ? "/settings/profile" : "/login"}
            >
              {mode === "verify" ? "Vai alle impostazioni" : "Vai al login"}
            </Link>
          </div>
        ) : mode !== "forgot" && !token ? (
          <div>
            <p role="alert">
              Apri il link ricevuto via email. Se è scaduto o lo hai già
              utilizzato, richiedine uno nuovo.
            </p>
            <Link
              href={
                mode === "verify" ? "/settings/profile" : "/forgot-password"
              }
              className="text-button"
            >
              Richiedi un nuovo link
            </Link>
          </div>
        ) : (
          <form
            ref={formRef}
            aria-busy={pending}
            onSubmit={submit}
            aria-label={
              mode === "forgot"
                ? "Recupero password"
                : mode === "reset"
                  ? "Nuova password"
                  : "Conferma email"
            }
          >
            {mode === "forgot" && (
              <div className="form-field">
                <label htmlFor="recovery-email">Email</label>
                <input
                  id="recovery-email"
                  name="email"
                  type="email"
                  required
                  maxLength={254}
                  autoComplete="email"
                  disabled={pending}
                  aria-invalid={!!fields.email}
                  aria-describedby={
                    fields.email ? "recovery-email-error" : undefined
                  }
                />
                {fields.email && (
                  <p id="recovery-email-error" className="error-text">
                    {fields.email}
                  </p>
                )}
              </div>
            )}
            {mode === "reset" && (
              <>
                {[
                  ["password", "Nuova password"],
                  ["confirmation", "Conferma password"],
                ].map(([name, label]) => (
                  <div className="form-field" key={name}>
                    <label htmlFor={`recovery-${name}`}>{label}</label>
                    <input
                      id={`recovery-${name}`}
                      name={name}
                      type="password"
                      autoComplete="new-password"
                      required
                      minLength={12}
                      disabled={pending}
                      aria-invalid={!!fields[name]}
                      aria-describedby={`${name}-hint`}
                    />
                    <p
                      id={`${name}-hint`}
                      className={fields[name] ? "error-text" : "field-hint"}
                    >
                      {fields[name] || "Almeno 12 caratteri, massimo 128 byte."}
                    </p>
                  </div>
                ))}
              </>
            )}
            {error && (
              <p role="alert" className="form-error" tabIndex={-1}>
                {error}
              </p>
            )}
            <button
              className="button primary submit-button"
              disabled={pending}
              type="submit"
            >
              {pending
                ? "Un momento…"
                : mode === "forgot"
                  ? "Invia link di recupero"
                  : mode === "reset"
                    ? "Aggiorna password"
                    : "Conferma email"}
            </button>
          </form>
        )}
        <LocalMailbox />
        <p className="auth-switch">
          <Link href="/login">Torna al login</Link>
        </p>
      </section>
    </>
  );
}
