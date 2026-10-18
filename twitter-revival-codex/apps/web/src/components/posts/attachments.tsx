"use client";
/* eslint-disable @next/next/no-img-element -- Authenticated same-origin media must keep browser cookies and no-store responses. */
import { useState } from "react";
import type { Attachment } from "@/lib/posts";
import styles from "./posts.module.css";
export function Attachments({ items }: { items: Attachment[] }) {
  return (
    <div className={styles.images}>
      {items.map((item) => (
        <MediaImage key={item.id} item={item} />
      ))}
    </div>
  );
}
function MediaImage({ item }: { item: Attachment }) {
  const [failed, setFailed] = useState(false);
  if (failed)
    return (
      <p className={styles.unavailable}>
        Immagine non disponibile: {item.alt_text}
      </p>
    );
  return (
    <a
      href={item.url}
      target="_blank"
      rel="noreferrer noopener"
      aria-label={`Apri immagine: ${item.alt_text}`}
    >
      <img
        src={item.url}
        alt={item.alt_text}
        width={item.width}
        height={item.height}
        loading="lazy"
        onError={() => setFailed(true)}
      />
    </a>
  );
}
