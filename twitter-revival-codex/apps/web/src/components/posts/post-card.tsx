"use client";
import Link from "next/link";
import { useState } from "react";
import { api } from "@/lib/api";
import { postsChanged, type Post, type PostPreview } from "@/lib/posts";
import { UserAvatar, avatarInitials } from "@/components/profile/shared";
import { Icon } from "@/components/layout/icon";
import { PostComposer } from "./composer";
import { Attachments } from "./attachments";
import { ReportButton } from "@/components/safety/report-button";
import styles from "./posts.module.css";

export function Reference({
  post,
  kind,
}: {
  post: PostPreview | null;
  kind: "reply" | "quote";
}) {
  if (!post)
    return (
      <p className={styles.unavailable}>Post originale non disponibile.</p>
    );
  return (
    <Link
      className={kind === "quote" ? styles.quote : styles.parent}
      href={`/posts/${post.id}`}
    >
      <span className={styles.context}>
        {kind === "reply" ? "In risposta a " : "Citazione di "}
        <strong>@{post.author.handle}</strong>
      </span>
      <p className="post-text">{post.body}</p>
      {post.media.length > 0 && (
        <span className={styles.context}>
          {post.media.length}{" "}
          {post.media.length === 1 ? "immagine" : "immagini"} · Apri il post per
          vederle
        </span>
      )}
    </Link>
  );
}
export function PostCard({
  post,
  detail = false,
}: {
  post: Post;
  detail?: boolean;
}) {
  const [pending, setPending] = useState(false);
  const [error, setError] = useState("");
  const [mode, setMode] = useState<"edit" | "quote" | "delete" | null>(null);
  async function mutate(action: "likes" | "reposts" | "delete") {
    setPending(true);
    setError("");
    try {
      await api(
        action === "delete"
          ? `/posts/${post.id}`
          : `/posts/${post.id}/${action}`,
        {
          method:
            action === "delete" ||
            (action === "likes" ? post.liked : post.reposted)
              ? "DELETE"
              : "POST",
          body: {},
        },
      );
      setMode(null);
      postsChanged();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Operazione non riuscita.");
    } finally {
      setPending(false);
    }
  }
  const date = new Date(post.created_at).toLocaleString("it-IT", {
    dateStyle: "medium",
    timeStyle: "short",
  });
  return (
    <article
      className={`post-card ${styles.card} ${detail ? styles.detail : ""}`}
      aria-label={`Post di @${post.author.handle}`}
      data-testid={`post-${post.id}`}
    >
      <Link
        href={`/u/${post.author.handle}`}
        aria-label={`${avatarInitials(post.author.display_name)}: profilo di @${post.author.handle}`}
      >
        <UserAvatar
          name={post.author.display_name}
          style={post.author.avatar_style}
        />
      </Link>
      <div className="post-content">
        {post.community_id && (
          <p className={styles.context}>
            <Link
              className="text-button"
              href={`/communities/${post.community_id}`}
            >
              Community · {post.community_name || "Apri community"}
            </Link>
            <span className="muted"> · Solo per i membri</span>
          </p>
        )}
        <div className={`post-meta ${styles.meta}`}>
          <Link href={`/u/${post.author.handle}`}>
            <strong>{post.author.display_name}</strong>
          </Link>
          <span>@{post.author.handle}</span>
          <Link href={`/posts/${post.id}`} className="muted">
            <time dateTime={post.created_at}>{date}</time>
          </Link>
          {post.updated_at !== post.created_at && (
            <span className="muted">Modificato</span>
          )}
        </div>
        {post.reply_to_id && (
          <div className={styles.context}>
            {post.reply_to ? (
              <Link href={`/posts/${post.reply_to.id}`}>
                In risposta a @{post.reply_to.author.handle}
              </Link>
            ) : (
              "In risposta a un post non disponibile"
            )}
          </div>
        )}
        {detail ? (
          <p className="post-text">{post.body}</p>
        ) : (
          <Link className={styles.bodyLink} href={`/posts/${post.id}`}>
            <p className="post-text">{post.body}</p>
          </Link>
        )}
        <Attachments items={post.media} />
        {post.quote_of_id && <Reference post={post.quote_of} kind="quote" />}
        <div
          className={`post-actions ${styles.actions}`}
          aria-label="Interazioni del post"
        >
          <Link
            href={`/posts/${post.id}#reply`}
            aria-label={`Rispondi, ${post.replies_count} risposte`}
          >
            <Icon name="reply" size={18} />
            <span>{post.replies_count}</span>
          </Link>
          {!post.community_id && (
            <button
              disabled={pending}
              aria-label={`${post.reposted ? "Annulla repost" : "Ripubblica"}, ${post.reposts_count} repost`}
              aria-pressed={post.reposted}
              onClick={() => void mutate("reposts")}
            >
              <Icon name="repeat" size={18} />
              <span>{post.reposts_count}</span>
            </button>
          )}
          <button
            disabled={pending}
            aria-label={`${post.liked ? "Rimuovi mi piace" : "Mi piace"}, ${post.likes_count} mi piace`}
            aria-pressed={post.liked}
            onClick={() => void mutate("likes")}
          >
            <Icon name="heart" size={18} />
            <span>{post.likes_count}</span>
          </button>
          {!post.community_id && (
            <button
              disabled={pending}
              aria-label={`Cita, ${post.quotes_count} citazioni`}
              onClick={() => setMode(mode === "quote" ? null : "quote")}
            >
              <Icon name="share" size={18} />
              <span>{post.quotes_count}</span>
            </button>
          )}
        </div>
        {post.is_own && (
          <div className={styles.ownActions}>
            <button
              className="text-button"
              disabled={pending}
              onClick={() => setMode(mode === "edit" ? null : "edit")}
            >
              Modifica
            </button>
            <button
              className="text-button"
              disabled={pending}
              onClick={() => setMode(mode === "delete" ? null : "delete")}
            >
              Elimina
            </button>
          </div>
        )}
        {mode === "delete" && (
          <div
            className={styles.confirm}
            role="group"
            aria-label="Conferma eliminazione"
          >
            <p>
              Eliminare questo post? Non sarà più visibile. Le risposte
              resteranno senza il contenuto originale.
            </p>
            <button
              className="button primary"
              disabled={pending}
              onClick={() => void mutate("delete")}
            >
              Conferma eliminazione
            </button>
            <button
              className="text-button"
              disabled={pending}
              onClick={() => setMode(null)}
            >
              Annulla
            </button>
          </div>
        )}
        {mode === "edit" && (
          <PostComposer
            key={`edit-${post.id}`}
            edit={post}
            onDone={() => setMode(null)}
            onCancel={() => setMode(null)}
          />
        )}
        {mode === "quote" && !post.community_id && (
          <PostComposer
            key={`quote-${post.id}`}
            quoteOf={post}
            onDone={() => setMode(null)}
            onCancel={() => setMode(null)}
          />
        )}
        {error && (
          <p className="error-text" role="alert">
            {error}
          </p>
        )}
        {!post.is_own && <ReportButton type="post" id={post.id} />}
      </div>
    </article>
  );
}
