"use client";
import { useEffect, useState } from "react";
import { api } from "@/lib/api";
import { useResource } from "@/lib/use-resource";
import type { Attachment } from "@/lib/posts";
import styles from "./posts.module.css";
export function draftsChanged() {
  window.dispatchEvent(new Event("radura:media-drafts-changed"));
}
export function DraftImages({
  selected,
  disabled,
  onSelect,
}: {
  selected: Attachment[];
  disabled: boolean;
  onSelect: (image: Attachment) => void;
}) {
  const resource = useResource<{
    data: {
      media: (Attachment & { available_until: string; attachable: boolean })[];
    };
  }>("/media/drafts");
  const { refresh } = resource;
  const [pending, setPending] = useState("");
  const [error, setError] = useState("");
  useEffect(() => {
    const update = () => {
      void refresh();
    };
    window.addEventListener("radura:media-drafts-changed", update);
    return () =>
      window.removeEventListener("radura:media-drafts-changed", update);
  }, [refresh]);
  async function remove(id: string) {
    setPending(id);
    setError("");
    try {
      await api(`/media/${id}`, { method: "DELETE", body: {} });
      draftsChanged();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Rimozione non riuscita.");
    } finally {
      setPending("");
    }
  }
  const items = (resource.data?.data.media || []).filter(
    (item) => !selected.some((image) => image.id === item.id),
  );
  return (
    <div className={styles.drafts}>
      <h3>Immagini già caricate</h3>
      <p className="field-hint">
        Le bozze sono utilizzabili per 24 ore. Puoi recuperarle o eliminarle
        qui.
      </p>
      {resource.loading && (
        <p role="status" className="muted">
          Caricamento bozze…
        </p>
      )}
      {(error || resource.error) && (
        <p role="alert" className="error-text">
          {error || resource.error}
        </p>
      )}
      {items.map((item) => (
        <div className={styles.draftRow} key={item.id}>
          <span>{item.alt_text}</span>
          <button
            className="text-button"
            type="button"
            disabled={disabled || !!pending || !item.attachable}
            onClick={() => onSelect(item)}
          >
            Usa immagine
          </button>
          <button
            className="text-button"
            type="button"
            disabled={disabled || !!pending}
            onClick={() => void remove(item.id)}
          >
            Elimina bozza
          </button>
        </div>
      ))}
      {!resource.loading && !resource.error && items.length === 0 && (
        <p className="muted">Nessun’altra immagine in bozza.</p>
      )}
    </div>
  );
}
