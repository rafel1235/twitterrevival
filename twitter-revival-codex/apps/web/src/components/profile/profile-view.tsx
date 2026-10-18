"use client";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useState } from "react";
import { api } from "@/lib/api";
import { useResource } from "@/lib/use-resource";
import type { Profile } from "@/lib/profile";
import { useSession } from "@/components/auth/session-provider";
import { SessionGate, PageError, UserAvatar } from "./shared";
import { ConnectionList } from "./connections";
import { PostList } from "@/components/posts/post-list";
import styles from "./profile.module.css";

export function ProfileView({
  handle,
  kind,
}: {
  handle: string;
  kind?: "followers" | "following";
}) {
  const { user } = useSession();
  return (
    <SessionGate>
      <ProfileContent
        key={`${user?.id}:${handle}:${kind}`}
        handle={handle}
        kind={kind}
      />
    </SessionGate>
  );
}
function ProfileContent({
  handle,
  kind,
}: {
  handle: string;
  kind?: "followers" | "following";
}) {
  const router = useRouter();
  const resource = useResource<{ data: { user: Profile } }>(
    `/users/${encodeURIComponent(handle)}`,
  );
  const [pending, setPending] = useState(false);
  const [error, setError] = useState("");
  const [confirmBlock, setConfirmBlock] = useState(false);
  const profile = resource.data?.data.user;
  async function action(type: "follow" | "block") {
    if (!profile) return;
    setPending(true);
    setError("");
    try {
      await api<void>(`/users/${profile.id}/${type}`, {
        method: type === "follow" && profile.following ? "DELETE" : "POST",
        body: {},
      });
      if (type === "block") {
        router.push("/settings/blocks");
      } else await resource.refresh();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Operazione non riuscita.");
    } finally {
      setPending(false);
    }
  }
  if (resource.loading && !profile)
    return (
      <p className={styles.state} role="status">
        Caricamento profilo…
      </p>
    );
  if (resource.error || !profile)
    return (
      <>
        <header className="feed-header">
          <h1>Profilo non disponibile</h1>
        </header>
        <PageError
          message={resource.error || "Profilo non disponibile."}
          retry={() => void resource.refresh()}
        />
        <div className={styles.content}>
          <Link className="text-button" href="/settings/blocks">
            Gestisci gli utenti bloccati
          </Link>
          <br />
          <Link className="text-button" href="/people">
            Apri un altro profilo
          </Link>
        </div>
      </>
    );
  const base = `/u/${profile.handle}`;
  return (
    <>
      <header className="feed-header">
        <div>
          <h1>{profile.display_name}</h1>
          <p>@{profile.handle}</p>
        </div>
        <Link className="text-button" href="/people">
          Persone
        </Link>
      </header>
      <div className={styles.banner} aria-hidden="true">
        <span />
        <span />
      </div>
      <section
        className={styles.profile}
        aria-label={`Profilo di ${profile.display_name}`}
      >
        <div className={styles.profileTop}>
          <UserAvatar
            name={profile.display_name}
            style={profile.avatar_style}
            large
          />
          {profile.is_self ? (
            <Link className="button" href="/settings/profile">
              Modifica profilo
            </Link>
          ) : (
            <div className={styles.actions}>
              <button
                className={`button ${profile.following ? styles.outline : "primary"}`}
                onClick={() => void action("follow")}
                disabled={pending}
                aria-pressed={profile.following}
              >
                {pending
                  ? "Un momento…"
                  : profile.following
                    ? "Non seguire più"
                    : "Segui"}
              </button>
              <button
                className={styles.blockButton}
                onClick={() => setConfirmBlock(true)}
                disabled={pending}
              >
                Blocca
              </button>
            </div>
          )}
        </div>
        <h2>{profile.display_name}</h2>
        <p className="muted">
          @{profile.handle}{" "}
          {profile.followed_by && (
            <span className={styles.followsYou}>Ti segue</span>
          )}
        </p>
        {profile.bio && <p className={styles.bio}>{profile.bio}</p>}
        <div className={styles.details}>
          {profile.location_text && <span>{profile.location_text}</span>}
          {profile.website_url && (
            <a
              href={profile.website_url}
              target="_blank"
              rel="noopener noreferrer"
            >
              {profile.website_url}
            </a>
          )}
          <span>
            Iscrizione:{" "}
            {new Date(profile.created_at).toLocaleDateString("it-IT", {
              month: "long",
              year: "numeric",
            })}
          </span>
        </div>
        <div className={styles.counts}>
          <Link href={`${base}/following`}>
            <strong>{profile.following_count}</strong> Seguiti
          </Link>
          <Link href={`${base}/followers`}>
            <strong>{profile.followers_count}</strong> Follower
          </Link>
        </div>
        {error && (
          <p role="alert" className="error-text">
            {error}
          </p>
        )}
        {confirmBlock && (
          <div
            className={styles.confirm}
            role="group"
            aria-label="Conferma blocco"
          >
            <p>
              Bloccare @{profile.handle}? Rimuove i follow reciproci e nasconde
              entrambi i profili. Sbloccare non ripristina i follow.
            </p>
            <button
              className="button primary"
              disabled={pending}
              onClick={() => void action("block")}
            >
              Conferma blocco
            </button>
            <button
              className="text-button"
              disabled={pending}
              onClick={() => setConfirmBlock(false)}
            >
              Annulla
            </button>
          </div>
        )}
      </section>
      <nav className={styles.tabs} aria-label="Sezioni profilo">
        <Link href={base} aria-current={!kind ? "page" : undefined}>
          Post
        </Link>
        <Link
          href={`${base}/followers`}
          aria-current={kind === "followers" ? "page" : undefined}
        >
          Follower
        </Link>
        <Link
          href={`${base}/following`}
          aria-current={kind === "following" ? "page" : undefined}
        >
          Seguiti
        </Link>
      </nav>
      {kind ? (
        <ConnectionList
          key={`${profile.id}:${kind}`}
          path={`/users/${profile.id}/${kind}`}
          empty={
            kind === "followers"
              ? "Ancora nessun follower visibile."
              : "Ancora nessun account seguito visibile."
          }
        />
      ) : (
        <PostList
          key={profile.id}
          path={`/users/${profile.id}/posts`}
          empty="Ancora nessun post visibile."
          label={`Post di @${profile.handle}`}
        />
      )}
    </>
  );
}
