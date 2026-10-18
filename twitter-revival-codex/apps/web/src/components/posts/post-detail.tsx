"use client";
import Link from "next/link";
import { useEffect } from "react";
import { useSession } from "@/components/auth/session-provider";
import { SessionGate, PageError } from "@/components/profile/shared";
import { useResource } from "@/lib/use-resource";
import type { Post } from "@/lib/posts";
import { PostComposer } from "./composer";
import { PostCard, Reference } from "./post-card";
import { PostList } from "./post-list";
import styles from "./posts.module.css";

export function PostDetail({ id }: { id: string }) {
  const { user } = useSession();
  return (
    <SessionGate>
      <DetailContent key={`${user?.id}:${id}`} id={id} />
    </SessionGate>
  );
}
function DetailContent({ id }: { id: string }) {
  const resource = useResource<{ data: { post: Post } }>(
    `/posts/${encodeURIComponent(id)}`,
  );
  const { refresh } = resource;
  useEffect(() => {
    const update = () => {
      void refresh();
    };
    window.addEventListener("radura:posts-changed", update);
    return () => window.removeEventListener("radura:posts-changed", update);
  }, [refresh]);
  const post = resource.data?.data.post;
  return (
    <>
      <header className="feed-header">
        <div>
          <h1>Conversazione</h1>
          <p>Un pensiero, tante prospettive</p>
        </div>
        <Link href="/" className="text-button">
          Home
        </Link>
      </header>
      {resource.loading && !post ? (
        <p className={styles.state} role="status">
          Caricamento conversazione…
        </p>
      ) : resource.error || !post ? (
        <PageError
          message={resource.error || "Post non disponibile."}
          retry={() => void refresh()}
        />
      ) : (
        <>
          {post.reply_to_id && (
            <div className={styles.parentContext}>
              <Reference post={post.reply_to} kind="reply" />
            </div>
          )}
          <PostCard post={post} detail />
          <div id="reply" className={styles.replyComposer}>
            <h2>Partecipa alla conversazione</h2>
            <PostComposer replyTo={post} />
          </div>
          <PostList
            key={post.id}
            path={`/posts/${post.id}/replies`}
            empty="Ancora nessuna risposta visibile. Inizia tu."
            label="Risposte"
          />
        </>
      )}
    </>
  );
}
