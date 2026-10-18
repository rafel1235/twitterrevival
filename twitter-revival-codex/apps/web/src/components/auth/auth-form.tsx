"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useEffect, useState } from "react";
import { api, ApiError, type User } from "@/lib/api";
import { useSession } from "./session-provider";
import { Icon } from "@/components/layout/icon";
import { productName } from "@/components/layout/app-shell";
import { useResource } from "@/lib/use-resource";
import { useFormFocus } from "@/lib/use-form-focus";

export function AuthForm({ mode }: { mode: "login" | "register" }) {
  const register = mode === "register";
  const router = useRouter();
  const { user, loading, setUser } = useSession();
  const [pending, setPending] = useState(false);
  const [error, setError] = useState("");
  const [fields, setFields] = useState<Record<string, string>>({});
  const formRef = useFormFocus(pending, fields, error);
  const beta = useResource<{
    data: { invite_required: boolean; enrollment_open: boolean };
  }>("/beta");
  const [inviteToken, setInviteToken] = useState("");
  useEffect(() => {
    if (!register) return;
    const value = new URLSearchParams(window.location.hash.slice(1)).get(
      "invite",
    );
    if (value) {
      window.history.replaceState(null, "", window.location.pathname);
      queueMicrotask(() => setInviteToken(value));
    }
  }, [register]);
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
            invite_token: inviteToken,
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
            ? "Una piccola beta, su invito. Usa l’email a cui è destinato il tuo codice."
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
            ref={formRef}
            onSubmit={submit}
            aria-busy={pending}
            aria-label={register ? "Registrazione" : "Accesso"}
          >
            {register && (
              <>
                {beta.data?.data.invite_required && (
                  <div className="form-field">
                    <label htmlFor="invite_token">Codice invito</label>
                    <input
                      id="invite_token"
                      name="invite_token"
                      type="password"
                      autoComplete="off"
                      value={inviteToken}
                      onChange={(e) => setInviteToken(e.target.value)}
                      required
                      maxLength={43}
                      disabled={pending}
                      aria-invalid={!!fields.invite_token}
                      aria-describedby="invite-hint invite-error"
                    />
                    <p id="invite-hint" className="field-hint">
                      Il codice è monouso, legato alla tua email e ha una
                      scadenza.
                    </p>
                    <p id="invite-error" className="error-text">
                      {fields.invite_token}
                    </p>
                  </div>
                )}
                {beta.data && !beta.data.data.enrollment_open && (
                  <p role="status">
                    I nuovi ingressi alla beta sono in pausa. Gli account
                    esistenti possono accedere.
                  </p>
                )}
                {beta.error && (
                  <p role="alert" className="form-error">
                    Impossibile verificare gli inviti.{" "}
                    <button
                      type="button"
                      className="text-button"
                      onClick={() => void beta.refresh()}
                    >
                      Riprova
                    </button>
                  </p>
                )}
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
              <div className="form-error" role="alert" tabIndex={-1}>
                {error}
                {fields.credentials && <p>{fields.credentials}</p>}
              </div>
            )}
            <button
              type="submit"
              className="button primary submit-button"
              disabled={pending || (register && (!beta.data || !!beta.error))}
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
          Beta privata locale. Account, post e immagini sono reali. Le email di
          prova arrivano nella casella locale; non inserire dati sensibili.
        </p>
      </section>
    </>
  );
}
