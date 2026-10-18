"use client";
import Link from "next/link";
import { useState } from "react";
import { api, ApiError } from "@/lib/api";
import { useResource } from "@/lib/use-resource";
import type { Profile } from "@/lib/profile";
import { useSession } from "@/components/auth/session-provider";
import { SessionGate, PageError, UserAvatar, LocalMailbox } from "./shared";
import styles from "./profile.module.css";

export function ProfileEditor() {
  const { user } = useSession();
  return (
    <SessionGate>
      <EditorLoader key={user?.id} handle={user?.handle || ""} />
    </SessionGate>
  );
}
function EditorLoader({ handle }: { handle: string }) {
  const resource = useResource<{ data: { user: Profile } }>(
    `/users/${encodeURIComponent(handle)}`,
  );
  return (
    <>
      <header className="feed-header">
        <div>
          <h1>Il tuo profilo</h1>
          <p>Scegli come presentarti alla community</p>
        </div>
        <Link className="text-button" href="/settings/blocks">
          Utenti bloccati
        </Link>
      </header>
      {resource.error ? (
        <PageError
          message={resource.error}
          retry={() => void resource.refresh()}
        />
      ) : resource.data ? (
        <Editor initial={resource.data.data.user} />
      ) : (
        <p className={styles.state} role="status">
          Caricamento profilo…
        </p>
      )}
    </>
  );
}
function Editor({ initial }: { initial: Profile }) {
  const { refresh } = useSession();
  const [values, setValues] = useState({
    display_name: initial.display_name,
    bio: initial.bio,
    avatar_style: initial.avatar_style,
    location_text: initial.location_text,
    website_url: initial.website_url,
  });
  const [pending, setPending] = useState(false);
  const [error, setError] = useState("");
  const [fields, setFields] = useState<Record<string, string>>({});
  const [message, setMessage] = useState("");
  async function submit(event: React.FormEvent) {
    event.preventDefault();
    setError("");
    setFields({});
    setMessage("");
    const invalid: Record<string, string> = {};
    for (const [key, max] of [
      ["display_name", 50],
      ["bio", 160],
      ["location_text", 100],
    ] as const) {
      if (Array.from(values[key].trim()).length > max)
        invalid[key] = `Massimo ${max} caratteri.`;
    }
    if (Object.keys(invalid).length) {
      setFields(invalid);
      return;
    }
    setPending(true);
    try {
      const response = await api<{ data: { user: Profile } }>("/users/me", {
        method: "PATCH",
        body: values,
      });
      const saved = response.data.user;
      setValues({
        display_name: saved.display_name,
        bio: saved.bio,
        avatar_style: saved.avatar_style,
        location_text: saved.location_text,
        website_url: saved.website_url,
      });
      await refresh();
      setMessage("Profilo aggiornato.");
    } catch (err) {
      setError(
        err instanceof Error ? err.message : "Salvataggio non riuscito.",
      );
      if (err instanceof ApiError) setFields(err.fields);
    } finally {
      setPending(false);
    }
  }
  const update = (key: keyof typeof values, value: string) =>
    setValues((previous) => ({ ...previous, [key]: value }));
  return (
    <div className={styles.content}>
      <form
        onSubmit={submit}
        aria-label="Modifica profilo"
        className={styles.editForm}
      >
        <div className={styles.avatarPreview}>
          <UserAvatar
            name={values.display_name}
            style={values.avatar_style}
            large
          />
          <p>
            Un avatar a iniziali, tutto tuo.
            <br />
            <span className="muted">
              Le immagini arriveranno in uno sprint successivo.
            </span>
          </p>
        </div>
        <fieldset disabled={pending}>
          <legend>Colore avatar</legend>
          <div className={styles.palette}>
            {[
              ["sage", "Verde"],
              ["peach", "Pesca"],
              ["lavender", "Lavanda"],
              ["sky", "Cielo"],
            ].map(([value, label]) => (
              <label key={value}>
                <input
                  type="radio"
                  name="avatar_style"
                  value={value}
                  checked={values.avatar_style === value}
                  onChange={() => update("avatar_style", value)}
                />
                <span className={`avatar ${value}`}>{label}</span>
              </label>
            ))}
          </div>
        </fieldset>
        {(
          [
            ["display_name", "Nome visualizzato", 50],
            ["bio", "Bio", 160],
            ["location_text", "Località", 100],
            ["website_url", "Sito web", 2048],
          ] as const
        ).map(([name, label, max]) => (
          <div className="form-field" key={name}>
            <label htmlFor={`profile-${name}`}>{label}</label>
            {name === "bio" ? (
              <textarea
                id={`profile-${name}`}
                value={values[name]}
                onChange={(e) => update(name, e.target.value)}
                disabled={pending}
                rows={4}
                aria-invalid={!!fields[name]}
                aria-describedby={`${name}-hint`}
              />
            ) : (
              <input
                id={`profile-${name}`}
                type={name === "website_url" ? "url" : "text"}
                value={values[name]}
                required={name === "display_name"}
                onChange={(e) => update(name, e.target.value)}
                disabled={pending}
                aria-invalid={!!fields[name]}
                aria-describedby={`${name}-hint`}
              />
            )}
            <p
              id={`${name}-hint`}
              className={fields[name] ? "error-text" : "field-hint"}
            >
              {fields[name] ||
                (name === "website_url"
                  ? "Solo link http o https."
                  : `${Array.from(values[name]).length}/${max} caratteri`)}
            </p>
          </div>
        ))}
        <p className="field-hint">
          Handle ed email restano invariati in questo sprint.
        </p>
        {error && (
          <p role="alert" className="form-error">
            {error}
          </p>
        )}
        {message && (
          <p role="status" className={styles.success}>
            {message}
          </p>
        )}
        <button
          className="button primary submit-button"
          type="submit"
          disabled={pending}
        >
          {pending ? "Salvataggio…" : "Salva profilo"}
        </button>
        <Link className="text-button" href={`/u/${initial.handle}`}>
          Vai al profilo
        </Link>
      </form>
      <EmailSettings />
    </div>
  );
}
function EmailSettings() {
  const { user } = useSession();
  const [pending, setPending] = useState(false);
  const [message, setMessage] = useState("");
  const [error, setError] = useState("");
  async function request() {
    setPending(true);
    setMessage("");
    setError("");
    try {
      const response = await api<{ data: { message: string } }>(
        "/auth/verification/request",
        { body: {} },
      );
      setMessage(response.data.message);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Richiesta non riuscita.");
    } finally {
      setPending(false);
    }
  }
  return (
    <section className={styles.emailSettings} aria-label="Sicurezza account">
      <h2>La tua email</h2>
      <p>{user?.email}</p>
      <p className={user?.email_verified ? styles.success : "muted"}>
        {user?.email_verified
          ? "Email verificata"
          : "Email non ancora verificata"}
      </p>
      {!user?.email_verified && (
        <button
          className="button"
          onClick={() => void request()}
          disabled={pending}
        >
          {pending ? "Richiesta…" : "Invia link di verifica"}
        </button>
      )}
      {message && <p role="status">{message}</p>}
      {error && (
        <p role="alert" className="error-text">
          {error}
        </p>
      )}
      <LocalMailbox />
      <Link href="/forgot-password" className="text-button">
        Reimposta la password tramite email
      </Link>
    </section>
  );
}
