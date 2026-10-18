"use client";
import { useId, useState } from "react";
import { api, ApiError } from "@/lib/api";
import {
  postsChanged,
  postBodyError,
  normalizePostBody,
  type Post,
  type PostPreview,
} from "@/lib/posts";
import { useSession } from "@/components/auth/session-provider";
import { UserAvatar } from "@/components/profile/shared";
import styles from "./posts.module.css";

export function PostComposer({
  replyTo,
  quoteOf,
  edit,
  onDone,
  onCancel,
}: {
  replyTo?: PostPreview;
  quoteOf?: PostPreview;
  edit?: Post;
  onDone?: (post: Post) => void;
  onCancel?: () => void;
}) {
  const { user } = useSession();
  const id = useId();
  const [body, setBody] = useState(edit?.body || "");
  const [pending, setPending] = useState(false);
  const [error, setError] = useState("");
  const [success, setSuccess] = useState("");
  const count = Array.from(normalizePostBody(body)).length;
  const label = edit
    ? "Modifica post"
    : replyTo
      ? "Risposta"
      : quoteOf
        ? "Citazione"
        : "Nuovo post";
  const action = edit
    ? "Salva modifica"
    : replyTo
      ? "Rispondi"
      : quoteOf
        ? "Pubblica citazione"
        : "Pubblica";
  async function submit(event: React.FormEvent) {
    event.preventDefault();
    setSuccess("");
    const invalid = postBodyError(body);
    if (invalid) {
      setError(invalid);
      return;
    }
    setPending(true);
    setError("");
    try {
      const result = await api<{ data: { post: Post } }>(
        edit ? `/posts/${edit.id}` : "/posts",
        {
          method: edit ? "PATCH" : "POST",
          body: edit
            ? { body: normalizePostBody(body) }
            : {
                body: normalizePostBody(body),
                ...(replyTo ? { reply_to_id: replyTo.id } : {}),
                ...(quoteOf ? { quote_of_id: quoteOf.id } : {}),
              },
        },
      );
      if (!edit) setBody("");
      setSuccess(edit ? "Modifica salvata." : "Post pubblicato.");
      postsChanged();
      onDone?.(result.data.post);
    } catch (err) {
      setError(
        err instanceof ApiError
          ? err.fields.body || err.message
          : "Pubblicazione non riuscita. Il testo è rimasto nel form.",
      );
    } finally {
      setPending(false);
    }
  }
  if (!user) return null;
  return (
    <form
      className={`composer ${styles.composer}`}
      aria-label={label}
      onSubmit={submit}
    >
      <UserAvatar name={user.display_name} style={user.avatar_style} />
      <div className="composer-body">
        {(replyTo || quoteOf) && (
          <p className={styles.context}>
            {replyTo ? "In risposta a" : "Citando"} @
            {(replyTo || quoteOf)!.author.handle}
          </p>
        )}
        <label htmlFor={id} className="sr-only">
          {label}
        </label>
        <textarea
          id={id}
          value={body}
          onChange={(event) => {
            setBody(event.target.value);
            setSuccess("");
          }}
          rows={3}
          placeholder={
            replyTo
              ? "Aggiungi la tua risposta…"
              : quoteOf
                ? "Aggiungi il tuo punto di vista…"
                : "Cosa ti passa per la testa?"
          }
          disabled={pending}
          aria-invalid={!!error}
          aria-describedby={`${id}-hint ${error ? `${id}-error` : ""}`}
        />
        <div className="composer-bottom">
          <span
            id={`${id}-hint`}
            className={count > 280 ? "error-text" : "muted"}
          >
            {count}/280
          </span>
          <div className={styles.composerActions}>
            {onCancel && (
              <button
                className="text-button"
                type="button"
                disabled={pending}
                onClick={onCancel}
              >
                Annulla
              </button>
            )}
            <button
              className="button primary"
              disabled={pending || count === 0 || count > 280}
            >
              {pending ? "Salvataggio…" : action}
            </button>
          </div>
        </div>
        {error && (
          <p id={`${id}-error`} className="error-text" role="alert">
            {error}
          </p>
        )}
        {success && (
          <p className="form-success" role="status">
            {success}
          </p>
        )}
      </div>
    </form>
  );
}
