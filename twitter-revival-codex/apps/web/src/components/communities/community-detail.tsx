"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import { useSession } from "@/components/auth/session-provider";
import {
  PageError,
  SessionGate,
  UserAvatar,
} from "@/components/profile/shared";
import { PostComposer } from "@/components/posts/composer";
import { PostList } from "@/components/posts/post-list";
import { ReportButton } from "@/components/safety/report-button";
import { api } from "@/lib/api";
import {
  communitiesChanged,
  useCommunityPage,
  type Community,
  type CommunityMember,
} from "@/lib/communities";
import { postsChanged } from "@/lib/posts";
import { useResource } from "@/lib/use-resource";
import { CommunityForm } from "./community-form";
import styles from "./communities.module.css";

export function CommunityDetail({ id }: { id: string }) {
  const { user } = useSession();
  return (
    <SessionGate>
      <Detail key={`${user?.id}:${id}`} id={id} />
    </SessionGate>
  );
}
function Detail({ id }: { id: string }) {
  const resource = useResource<{ data: { community: Community } }>(
    `/communities/${encodeURIComponent(id)}`,
  );
  const { refresh } = resource;
  const [membership, setMembership] = useState<{
    member: boolean;
    previous: Community | undefined;
  } | null>(null);
  const [revision, setRevision] = useState(0);
  const [pending, setPending] = useState(false);
  const [error, setError] = useState("");
  const [message, setMessage] = useState("");
  const [confirmLeave, setConfirmLeave] = useState(false);
  const [archived, setArchived] = useState(false);
  useEffect(() => {
    const update = () => void refresh();
    window.addEventListener("radura:communities-changed", update);
    window.addEventListener("focus", update);
    return () => {
      window.removeEventListener("radura:communities-changed", update);
      window.removeEventListener("focus", update);
    };
  }, [refresh]);
  const community = resource.data?.data.community;
  const isMember =
    membership && membership.previous === community
      ? membership.member
      : (community?.is_member ?? false);
  async function changeMembership(join: boolean) {
    if (pending) return;
    setPending(true);
    setError("");
    setMessage("");
    try {
      await api(`/communities/${encodeURIComponent(id)}/membership`, {
        method: join ? "POST" : "DELETE",
        body: {},
      });
      setMembership({ member: join, previous: community });
      setRevision((previous) => previous + 1);
      setConfirmLeave(false);
      setMessage(
        join
          ? "Ora fai parte della community."
          : "Hai abbandonato la community. Non puoi più leggere i suoi post.",
      );
      communitiesChanged();
      postsChanged();
    } catch (err) {
      setError(
        err instanceof Error
          ? err.message
          : "Impossibile aggiornare l’iscrizione.",
      );
    } finally {
      setPending(false);
    }
  }
  if (archived)
    return (
      <section className={styles.state}>
        <h1>Community archiviata</h1>
        <p>
          I post e le conversazioni di questa community non sono più
          accessibili.
        </p>
        <Link href="/communities" className="text-button">
          Torna alle community
        </Link>
      </section>
    );
  return (
    <>
      <header className={`feed-header ${styles.detailHeader}`}>
        <div>
          <h1>{community?.name || "Community"}</h1>
          <p>Community pubblica nella beta</p>
        </div>
        <Link href="/communities" className="text-button">
          Tutte le community
        </Link>
      </header>
      {resource.loading && !community && (
        <p className={styles.state} role="status">
          Caricamento community…
        </p>
      )}
      {resource.error && (
        <PageError
          message={resource.error}
          retry={() => void refresh()}
          pending={resource.loading}
          stale={!!community}
        />
      )}
      {community && (
        <>
          <section
            className={styles.content}
            aria-label="Informazioni sulla community"
          >
            <p className={styles.meta}>
              {community.members_count}{" "}
              {community.members_count === 1 ? "membro" : "membri"} · Creata il{" "}
              {new Date(community.created_at).toLocaleDateString("it-IT")}
            </p>
            <p className={styles.details}>
              {community.description ||
                "Uno spazio aperto per una passione comune."}
            </p>
            <h2>Regole della community</h2>
            <p className={styles.details}>
              {community.rules ||
                "Rispetta le altre persone e le regole di Radura."}
            </p>
            <div className={styles.notice}>
              <p>
                <strong>Solo i membri possono leggere e pubblicare.</strong>
              </p>
              <p>
                I post restano nella community e non vengono mostrati nella
                home, nei profili o nella ricerca generale.
              </p>
              <p>
                Nome, descrizione, regole ed elenco dei membri sono visibili
                alle persone della beta; l’iscrizione è libera. I moderatori di
                Radura possono esaminare i post segnalati per gestire gli abusi.
              </p>
            </div>
            {!community.is_owner && (
              <div>
                <p className="field-hint">
                  Per problemi con il nome o le regole, segnala il profilo del
                  proprietario e indica la community: {community.name} (
                  {community.id}).
                </p>
                <ReportButton type="user" id={community.owner_id} />
              </div>
            )}
            {isMember ? (
              <>
                <p className={styles.badge}>
                  {community.is_owner
                    ? "Sei proprietario e membro della community"
                    : "Sei membro della community"}
                </p>
                {community.is_owner ? (
                  <p className="field-hint">
                    Come proprietario puoi archiviare la community. Finché è
                    attiva, non puoi abbandonarla.
                  </p>
                ) : (
                  <button
                    className="text-button"
                    disabled={pending}
                    aria-expanded={confirmLeave}
                    aria-controls="community-leave"
                    onClick={() => setConfirmLeave(!confirmLeave)}
                  >
                    Abbandona community
                  </button>
                )}
                {confirmLeave && !community.is_owner && (
                  <div
                    className={styles.danger}
                    id="community-leave"
                    role="group"
                    aria-label="Conferma uscita dalla community"
                  >
                    <p>
                      Abbandonando la community non potrai più leggere i suoi
                      post. I post che hai già pubblicato resteranno nella
                      community. Puoi iscriverti nuovamente.
                    </p>
                    <div className={styles.actions}>
                      <button
                        className="button primary"
                        disabled={pending}
                        onClick={() => void changeMembership(false)}
                      >
                        {pending ? "Uscita…" : "Conferma uscita"}
                      </button>
                      <button
                        className="text-button"
                        disabled={pending}
                        onClick={() => setConfirmLeave(false)}
                      >
                        Annulla
                      </button>
                    </div>
                  </div>
                )}
              </>
            ) : (
              <button
                className="button primary"
                disabled={pending || resource.loading}
                onClick={() => void changeMembership(true)}
              >
                {pending ? "Iscrizione…" : "Iscriviti alla community"}
              </button>
            )}
            {error && (
              <p className="form-error" role="alert">
                {error}
              </p>
            )}
            {message && (
              <p className="form-success" role="status">
                {message}
              </p>
            )}
          </section>
          {community.can_manage && (
            <ManageCommunity
              community={community}
              onArchived={() => {
                setArchived(true);
                setMembership(null);
                communitiesChanged();
                postsChanged();
              }}
            />
          )}
          {isMember ? (
            <section
              key={`feed-${revision}`}
              aria-label="Conversazioni della community"
            >
              <div className={styles.content}>
                <h2>Conversazioni</h2>
                <p className="muted">Solo per i membri di {community.name}</p>
              </div>
              <PostComposer
                community={{ id: community.id, name: community.name }}
              />
              <PostList
                path={`/communities/${encodeURIComponent(id)}/posts`}
                label="Post della community"
                empty="Ancora nessun post in questa community. Inizia una conversazione."
              />
            </section>
          ) : (
            <section
              className={styles.notice}
              aria-label="Post riservati ai membri"
            >
              <h2>Entra nella conversazione</h2>
              <p>
                Iscriviti per leggere i post della community e pubblicare il tuo
                primo pensiero.
              </p>
            </section>
          )}
          <CommunityMembers key={`members-${revision}`} id={id} />
        </>
      )}
    </>
  );
}

function CommunityMembers({ id }: { id: string }) {
  const list = useCommunityPage<CommunityMember>(
    `/communities/${encodeURIComponent(id)}/members`,
  );
  return (
    <section aria-label="Membri della community" aria-busy={list.pending}>
      <div className={`${styles.content} ${styles.heading}`}>
        <h2>Membri</h2>
        <button
          className="text-button"
          disabled={list.pending}
          onClick={() => void list.load()}
        >
          Aggiorna membri
        </button>
      </div>
      {list.error && (
        <PageError
          message={list.error}
          pending={list.pending}
          stale={list.data.items.length > 0}
          retry={list.retry}
        />
      )}
      {list.pending && (
        <p className={styles.state} role="status">
          Caricamento membri…
        </p>
      )}
      {!list.pending && !list.error && !list.data.items.length && (
        <p className={styles.state}>Nessun membro visibile.</p>
      )}
      <ul className={styles.list}>
        {list.data.items.map((member) => (
          <li key={member.id}>
            <Link className={styles.member} href={`/u/${member.handle}`}>
              <UserAvatar
                name={member.display_name}
                style={member.avatar_style}
                small
              />
              <div>
                <strong>{member.display_name}</strong>
                <span className="muted">@{member.handle}</span>
              </div>
            </Link>
          </li>
        ))}
      </ul>
      {list.data.next_cursor && (
        <div className={styles.state}>
          <button
            className="button primary"
            disabled={list.pending}
            onClick={() => void list.load(list.data.next_cursor)}
          >
            Carica altri membri
          </button>
        </div>
      )}
    </section>
  );
}

function ManageCommunity({
  community,
  onArchived,
}: {
  community: Community;
  onArchived: () => void;
}) {
  const [editing, setEditing] = useState(false);
  const [confirm, setConfirm] = useState(false);
  const [pending, setPending] = useState(false);
  const [error, setError] = useState("");
  const [saved, setSaved] = useState("");
  async function archive() {
    if (pending) return;
    setPending(true);
    setError("");
    try {
      await api(`/communities/${community.id}`, { method: "DELETE", body: {} });
      onArchived();
    } catch (err) {
      setError(
        err instanceof Error ? err.message : "Archiviazione non riuscita.",
      );
    } finally {
      setPending(false);
    }
  }
  return (
    <details className={styles.manage}>
      <summary>Gestisci community</summary>
      {community.is_owner && (
        <>
          <button
            className="text-button"
            aria-expanded={editing}
            aria-controls="community-edit"
            disabled={pending}
            onClick={() => {
              setEditing(!editing);
              setSaved("");
            }}
          >
            {editing ? "Chiudi modifica" : "Modifica community"}
          </button>
          {editing && (
            <div id="community-edit">
              <CommunityForm
                key={community.updated_at}
                initial={community}
                onCancel={() => setEditing(false)}
                onDone={() => {
                  setEditing(false);
                  setSaved("Community aggiornata.");
                }}
              />
            </div>
          )}
        </>
      )}
      {saved && (
        <p className="form-success" role="status">
          {saved}
        </p>
      )}
      <p>
        Archiviando la community la rimuovi dall’elenco e rendi inaccessibili i
        suoi post. Non è previsto il ripristino dall’interfaccia.
      </p>
      <button
        className="text-button"
        disabled={pending}
        aria-expanded={confirm}
        aria-controls="community-archive"
        onClick={() => setConfirm(!confirm)}
      >
        Archivia community
      </button>
      {confirm && (
        <div
          className={styles.danger}
          id="community-archive"
          role="group"
          aria-label="Conferma archiviazione community"
        >
          <p>
            Archiviare {community.name}? Tutti i membri perderanno l’accesso
            alle conversazioni.
          </p>
          <div className={styles.actions}>
            <button
              className="button primary"
              disabled={pending}
              onClick={() => void archive()}
            >
              {pending ? "Archiviazione…" : "Conferma archiviazione"}
            </button>
            <button
              className="text-button"
              disabled={pending}
              onClick={() => setConfirm(false)}
            >
              Annulla
            </button>
          </div>
        </div>
      )}
      {error && (
        <p className="form-error" role="alert">
          {error}
        </p>
      )}
    </details>
  );
}
