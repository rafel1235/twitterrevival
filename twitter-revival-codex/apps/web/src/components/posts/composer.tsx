"use client";
import { useId, useRef, useState } from "react";
import { api, ApiError } from "@/lib/api";
import {
  postsChanged,
  postBodyError,
  normalizePostBody,
  type Post,
  type PostPreview,
  type Attachment,
} from "@/lib/posts";
import { useSession } from "@/components/auth/session-provider";
import { UserAvatar } from "@/components/profile/shared";
import styles from "./posts.module.css";
import { Attachments } from "./attachments";
import { DraftImages, draftsChanged } from "./draft-images";

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
  const [images, setImages] = useState<Attachment[]>([]);
  const [file, setFile] = useState<File | null>(null);
  const [alt, setAlt] = useState("");
  const [uploading, setUploading] = useState(false);
  const [showUpload, setShowUpload] = useState(false);
  const fileInput = useRef<HTMLInputElement>(null);
  async function upload() {
    if (
      !file ||
      !["image/jpeg", "image/png"].includes(file.type) ||
      file.size > 5 * 1024 * 1024 ||
      !alt.trim() ||
      Array.from(alt.trim()).length > 200
    ) {
      setError(
        "Scegli un JPEG/PNG fino a 5 MiB e una descrizione di 1–200 caratteri.",
      );
      return;
    }
    setUploading(true);
    setError("");
    setSuccess("");
    try {
      const form = new FormData();
      form.set("file", file);
      form.set("alt_text", alt.trim());
      const result = await api<{ data: { media: Attachment } }>("/media", {
        form,
      });
      setImages((previous) => [...previous, result.data.media]);
      draftsChanged();
      setFile(null);
      setAlt("");
      if (fileInput.current) fileInput.current.value = "";
    } catch (err) {
      setError(
        err instanceof Error ? err.message : "Caricamento non riuscito.",
      );
    } finally {
      setUploading(false);
    }
  }
  async function removeImage(image: Attachment) {
    setUploading(true);
    setError("");
    try {
      await api(`/media/${image.id}`, { method: "DELETE", body: {} });
      setImages((previous) => previous.filter((item) => item.id !== image.id));
      draftsChanged();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Rimozione non riuscita.");
    } finally {
      setUploading(false);
    }
  }
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
    if (pending || uploading) return;
    if (file) {
      setError(
        "Carica l'immagine selezionata oppure annulla la selezione prima di pubblicare.",
      );
      return;
    }
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
                media_ids: images.map((image) => image.id),
              },
        },
      );
      if (!edit) setBody("");
      setImages([]);
      setFile(null);
      setAlt("");
      setShowUpload(false);
      if (fileInput.current) fileInput.current.value = "";
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
        {images.map((image) => (
          <div className={styles.uploadPreview} key={image.id}>
            <Attachments items={[image]} />
            <button
              className="text-button"
              type="button"
              disabled={pending || uploading}
              onClick={() => void removeImage(image)}
            >
              Rimuovi immagine: {image.alt_text}
            </button>
          </div>
        ))}
        {!edit && (
          <button
            className="text-button"
            type="button"
            disabled={pending || uploading || images.length >= 4}
            aria-expanded={showUpload}
            onClick={() => setShowUpload(!showUpload)}
          >
            Aggiungi immagine ({images.length}/4)
          </button>
        )}
        {showUpload && !edit && images.length < 4 && (
          <div
            className={styles.uploadBox}
            role="group"
            aria-label="Allega immagine"
          >
            <label htmlFor={`${id}-file`}>Immagine JPEG o PNG</label>
            <input
              ref={fileInput}
              id={`${id}-file`}
              type="file"
              accept="image/jpeg,image/png"
              disabled={pending || uploading}
              onChange={(event) => setFile(event.target.files?.[0] || null)}
            />
            <label htmlFor={`${id}-alt`}>Descrizione dell’immagine</label>
            <input
              id={`${id}-alt`}
              type="text"
              value={alt}
              onChange={(event) => setAlt(event.target.value)}
              disabled={pending || uploading}
            />
            <p className="field-hint">
              Fino a 5 MiB, 4096 px per lato e 8 megapixel. Aggiungi una
              descrizione per chi usa un lettore di schermo. Il post richiede
              anche un testo.
            </p>
            {file && (
              <button
                className="text-button"
                type="button"
                disabled={pending || uploading}
                onClick={() => {
                  setFile(null);
                  if (fileInput.current) fileInput.current.value = "";
                }}
              >
                Annulla selezione
              </button>
            )}
            <button
              className="button primary"
              type="button"
              disabled={pending || uploading || !file}
              onClick={() => void upload()}
            >
              {uploading ? "Caricamento…" : "Carica immagine"}
            </button>
            <DraftImages
              selected={images}
              disabled={pending || uploading || images.length >= 4}
              onSelect={(image) =>
                setImages((previous) => [...previous, image])
              }
            />
          </div>
        )}
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
                disabled={pending || uploading}
                onClick={onCancel}
              >
                Annulla
              </button>
            )}
            <button
              className="button primary"
              disabled={
                pending || uploading || !!file || count === 0 || count > 280
              }
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
