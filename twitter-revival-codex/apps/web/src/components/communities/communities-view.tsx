"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useState } from "react";
import { useSession } from "@/components/auth/session-provider";
import { SessionGate, PageError } from "@/components/profile/shared";
import { useCommunityPage, type Community } from "@/lib/communities";
import { CommunityForm } from "./community-form";
import styles from "./communities.module.css";

export function CommunitiesView() {
  const { user } = useSession();
  return (
    <SessionGate>
      <Directory key={user?.id} />
    </SessionGate>
  );
}
function Directory() {
  const router = useRouter();
  const [mine, setMine] = useState(false);
  const [creating, setCreating] = useState(false);
  return (
    <>
      <header className="feed-header">
        <div>
          <h1>Community</h1>
          <p>Trova persone con cui condividere una passione</p>
        </div>
      </header>
      <section className={styles.content} aria-label="Scopri le community">
        <div className={styles.heading}>
          <h2>Un interesse, uno spazio comune</h2>
          <button
            className="button primary"
            aria-expanded={creating}
            aria-controls="community-create"
            onClick={() => setCreating(!creating)}
          >
            {creating ? "Chiudi creazione" : "Crea una community"}
          </button>
        </div>
        <p>
          Le community sono pubbliche all’interno della beta: puoi scoprirle e
          iscriverti liberamente. Solo i membri possono leggere e pubblicare i
          post.
        </p>
        {creating && (
          <div id="community-create">
            <CommunityForm
              onCancel={() => setCreating(false)}
              onDone={(community) =>
                router.push(`/communities/${community.id}`)
              }
            />
          </div>
        )}
      </section>
      <div className={styles.tabs} role="group" aria-label="Filtra community">
        <button
          className={`button ${styles.tab}`}
          aria-pressed={!mine}
          onClick={() => setMine(false)}
        >
          Tutte le community
        </button>
        <button
          className={`button ${styles.tab}`}
          aria-pressed={mine}
          onClick={() => setMine(true)}
        >
          Le mie community
        </button>
      </div>
      <DirectoryList key={String(mine)} mine={mine} />
    </>
  );
}
function DirectoryList({ mine }: { mine: boolean }) {
  const list = useCommunityPage<Community>(
    `/communities${mine ? "?mine=1" : ""}`,
  );
  return (
    <section
      aria-label={mine ? "Le tue community" : "Community disponibili"}
      aria-busy={list.pending}
    >
      <div className={styles.content}>
        <button
          className="text-button"
          disabled={list.pending}
          onClick={() => void list.load()}
        >
          Aggiorna community
        </button>
      </div>
      {list.error && (
        <PageError
          message={list.error}
          stale={list.data.items.length > 0}
          pending={list.pending}
          retry={list.retry}
        />
      )}
      {list.pending && (
        <p className={styles.state} role="status">
          Caricamento community…
        </p>
      )}
      {!list.pending && !list.error && !list.data.items.length && (
        <p className={styles.state}>
          {mine
            ? "Non partecipi ancora a nessuna community. Esplora le community disponibili o creane una."
            : "Non ci sono ancora community. Crea il primo spazio di conversazione."}
        </p>
      )}
      <ul className={styles.list}>
        {list.data.items.map((community) => (
          <li className={styles.card} key={community.id}>
            <Link href={`/communities/${community.id}`}>
              <h2>{community.name}</h2>
              {community.description && <p>{community.description}</p>}
              <p className={styles.meta}>
                {community.members_count}{" "}
                {community.members_count === 1 ? "membro" : "membri"} ·
                Community pubblica
              </p>
              {community.is_member && (
                <p>
                  <span className={styles.badge}>
                    {community.is_owner ? "La tua community" : "Sei membro"}
                  </span>
                </p>
              )}
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
            Carica altre community
          </button>
        </div>
      )}
    </section>
  );
}
