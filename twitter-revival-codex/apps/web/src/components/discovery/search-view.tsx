"use client";
import Link from "next/link";
import { useId, useState } from "react";
import { useSession } from "@/components/auth/session-provider";
import { SessionGate } from "@/components/profile/shared";
import { ConnectionList } from "@/components/profile/connections";
import { PostList } from "@/components/posts/post-list";
import styles from "./discovery.module.css";

export function SearchBox({
  initial = "",
  type = "posts",
  compact = false,
}: {
  initial?: string;
  type?: string;
  compact?: boolean;
}) {
  const id = useId();
  const [term, setTerm] = useState(initial);
  const [error, setError] = useState("");
  return (
    <form
      action="/search"
      method="get"
      role="search"
      aria-label={compact ? "Ricerca rapida" : "Ricerca"}
      className={compact ? styles.compactSearch : styles.searchForm}
      onSubmit={(event) => {
        const n = Array.from(term.trim()).length;
        if (n < 2 || n > 100) {
          event.preventDefault();
          setError("Cerca da 2 a 100 caratteri.");
        }
      }}
    >
      <label htmlFor={id} className={compact ? "sr-only" : ""}>
        Cerca in Radura
      </label>
      <div>
        <input
          id={id}
          name="q"
          value={term}
          onChange={(event) => setTerm(event.target.value)}
          type="search"
          placeholder="Post, persone, idee…"
          aria-describedby={error ? `${id}-error` : undefined}
        />
        <input type="hidden" name="type" value={type} />
        <button className="button primary">Cerca</button>
      </div>
      {error && (
        <p id={`${id}-error`} className="error-text" role="alert">
          {error}
        </p>
      )}
    </form>
  );
}
export function SearchView({ query, type }: { query: string; type: string }) {
  const { user } = useSession();
  const kind = type === "users" ? "users" : "posts";
  const path = `/search?q=${encodeURIComponent(query)}&type=${kind}`;
  return (
    <SessionGate>
      <div key={`${user?.id}:${query}:${kind}`}>
        <header className="feed-header">
          <div>
            <h1>Esplora</h1>
            <p>Trova pensieri e persone nella radura</p>
          </div>
        </header>
        <SearchBox initial={query} type={kind} />
        <nav className={styles.tabs} aria-label="Tipo di ricerca">
          <Link
            href={`/search?q=${encodeURIComponent(query)}&type=posts`}
            aria-current={kind === "posts" ? "page" : undefined}
          >
            Post
          </Link>
          <Link
            href={`/search?q=${encodeURIComponent(query)}&type=users`}
            aria-current={kind === "users" ? "page" : undefined}
          >
            Persone
          </Link>
        </nav>
        {query ? (
          kind === "posts" ? (
            <PostList
              key={path}
              path={path}
              empty="Nessun post visibile corrisponde alla ricerca."
              label="Risultati post"
            />
          ) : (
            <ConnectionList
              key={path}
              path={path}
              empty="Nessuna persona visibile corrisponde alla ricerca."
              label="Risultati persone"
            />
          )
        ) : (
          <div className={styles.state}>
            <h2>Segui la curiosità.</h2>
            <p>
              Cerca parole nei post, un handle o un nome. I risultati rispettano
              i blocchi.
            </p>
          </div>
        )}
      </div>
    </SessionGate>
  );
}
