"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useState } from "react";
import { api, ApiError, type User } from "@/lib/api";
import { useSession } from "./session-provider";
import { Icon } from "@/components/layout/icon";
import { productName } from "@/components/layout/app-shell";

export function AuthForm({ mode }: { mode: "login" | "register" }) {
  const register = mode === "register";
  const router = useRouter();
  const { user, loading, setUser } = useSession();
  const [pending, setPending] = useState(false);
  const [error, setError] = useState("");
  const [fields, setFields] = useState<Record<string, string>>({});
  async function submit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setError("");
    setFields({});
    const form = new FormData(event.currentTarget);
    const password = String(form.get("password"));
    if (
      Array.from(password).length < 12 ||
      new TextEncoder().encode(password).length > 128
    ) {
      setFields({ password: "Usa almeno 12 caratteri e al massimo 128 byte." });
      return;
    }
    const body = {
      email: String(form.get("email")),
      password,
      ...(register
        ? {
            handle: String(form.get("handle")),
            display_name: String(form.get("display_name")),
          }
        : {}),
    };
    setPending(true);
    try {
      const response = await api<{ data: { user: User } }>(`/auth/${mode}`, {
        body,
      });
      setUser(response.data.user);
      router.replace("/");
    } catch (err) {
      if (err instanceof ApiError) {
        setError(err.message);
        setFields(err.fields);
      } else setError("Richiesta non riuscita. Riprova.");
    } finally {
      setPending(false);
    }
  }
  const field = (name: string, label: string, type = "text", extra = {}) => (
    <div className="form-field">
      <label htmlFor={name}>{label}</label>
      <input
        id={name}
        name={name}
        type={type}
        required
        disabled={pending}
        aria-invalid={!!fields[name]}
        aria-describedby={
          fields[name]
            ? `${name}-error`
            : name === "password"
              ? "password-hint"
              : undefined
        }
        {...extra}
      />
      {fields[name] && (
        <p id={`${name}-error`} className="error-text">
          {fields[name]}
        </p>
      )}
    </div>
  );
  return (
    <>
      <header className="feed-header">
        <div>
          <h1>{register ? "Un nuovo inizio" : "Bentornato"}</h1>
          <p>Il tuo spazio, le tue conversazioni</p>
        </div>
        <Icon name="leaf" />
      </header>
      <section className="auth-card">
        <span className="auth-mark">
          <Icon name="leaf" size={36} />
        </span>
        <span className="eyebrow">{productName.toUpperCase()}</span>
        <h2>
          {register ? "Mettiamo radici, insieme." : "Ritrova la tua radura."}
        </h2>
        <p className="auth-intro">
          {register
            ? "Crea il tuo account. Le idee avranno presto un posto dove incontrarsi."
            : "Accedi con l’email e la password del tuo account."}
        </p>
        {loading ? (
          <p role="status">Verifica sessione…</p>
        ) : user ? (
          <div className="signed-in">
            <p>
              Hai effettuato l’accesso come <strong>{user.display_name}</strong>{" "}
              (@{user.handle}).
            </p>
            <Link href="/" className="button primary">
              Vai alla home
            </Link>
          </div>
        ) : (
          <form
            onSubmit={submit}
            aria-label={register ? "Registrazione" : "Accesso"}
          >
            {register && (
              <>
                {field("display_name", "Come ti chiami?", "text", {
                  autoComplete: "nickname",
                })}
                {field("handle", "Handle", "text", {
                  pattern: "[A-Za-z0-9_]{3,20}",
                  minLength: 3,
                  maxLength: 20,
                  autoComplete: "username",
                  placeholder: "il_tuo_nome",
                  autoCapitalize: "none",
                  spellCheck: false,
                })}
                <p className="field-hint">
                  3–20 lettere ASCII, cifre o underscore. Non distingue
                  maiuscole e minuscole.
                </p>
              </>
            )}
            {field("email", "Email", "email", {
              autoComplete: "email",
              maxLength: 254,
              autoCapitalize: "none",
              spellCheck: false,
            })}
            {field("password", "Password", "password", {
              autoComplete: register ? "new-password" : "current-password",
              minLength: 12,
            })}
            <p className="field-hint" id="password-hint">
              Almeno 12 caratteri, massimo 128 byte. Usa una password unica.
            </p>
            {!register && (
              <Link href="/forgot-password" className="text-button">
                Password dimenticata?
              </Link>
            )}
            {error && (
              <div className="form-error" role="alert">
                {error}
                {fields.credentials && <p>{fields.credentials}</p>}
              </div>
            )}
            <button
              type="submit"
              className="button primary submit-button"
              disabled={pending}
            >
              {pending ? "Un momento…" : register ? "Crea account" : "Accedi"}
              <Icon name="arrow" size={18} />
            </button>
            <p className="auth-switch">
              {register ? "Hai già un account?" : "Non hai ancora un account?"}{" "}
              <Link href={register ? "/login" : "/register"}>
                {register ? "Accedi" : "Registrati"}
              </Link>
            </p>
          </form>
        )}
        <p className="auth-footnote">
          Anteprima Sprint 2. Gli account sono reali; feed e pubblicazione sono
          ancora dimostrativi. Non inserire dati sensibili in questo ambiente di
          sviluppo.
        </p>
      </section>
    </>
  );
}
