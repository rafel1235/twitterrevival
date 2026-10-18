"use client";

import Link from "next/link";
import { useSession } from "@/components/auth/session-provider";
import { Icon } from "@/components/layout/icon";
import { HomeTimeline } from "@/components/posts/home-timeline";

const posts = [
  {
    name: "Ada Riva",
    handle: "ada_idee",
    initials: "AR",
    color: "peach",
    time: "09:41",
    text: "Le cose belle cominciano spesso con una domanda semplice: e se ci fosse un modo diverso?\n\nOggi parto da qui. Un taccuino, un caffè e un po’ di curiosità.",
    tag: "piccole partenze",
  },
  {
    name: "Luca Serra",
    handle: "luca_fuori",
    initials: "LS",
    color: "sage",
    time: "09:18",
    text: "Promemoria gentile: alzare gli occhi dallo schermo ogni tanto. La luce di ottobre sta facendo del suo meglio. 🍂",
    tag: null,
  },
  {
    name: "Studio Orizzonte",
    handle: "orizzonte",
    initials: "SO",
    color: "lavender",
    time: "08:56",
    text: "Ci piacciono gli spazi digitali che lasciano respirare le idee. Meno fretta, più ascolto.\n\nQual è l’ultima conversazione che vi ha fatto cambiare prospettiva?",
    tag: "idee in circolo",
  },
];

export function Timeline() {
  const { user, loading } = useSession();
  if (loading)
    return (
      <p className="feed-end" role="status">
        Caricamento timeline…
      </p>
    );
  return user ? <HomeTimeline key={user.id} /> : <DemoTimeline />;
}
function DemoTimeline() {
  return (
    <>
      <header className="feed-header">
        <div>
          <h1>Home</h1>
          <p>Uno sguardo alla tua radura</p>
        </div>
        <span className="header-symbol">
          <Icon name="spark" />
        </span>
      </header>
      <div className="feed-tab">
        <strong>La tua timeline</strong>
        <span className="demo-badge">DEMO</span>
      </div>
      <section
        className="composer"
        aria-label="Composer dimostrativo: accedi per pubblicare"
      >
        <span className="avatar sage" aria-hidden="true">
          <Icon name="leaf" />
        </span>
        <div className="composer-body">
          <label className="sr-only" htmlFor="post-body">
            Nuovo post, accedi per pubblicare
          </label>
          <textarea
            id="post-body"
            placeholder="Cosa ti passa per la testa?"
            disabled
            rows={2}
          />
          <div className="composer-bottom">
            <span>
              <Icon name="image" size={20} />
              <small>Accedi per pubblicare</small>
            </span>
            <button className="button primary" disabled>
              Pubblica
            </button>
          </div>
        </div>
      </section>
      <div className="demo-notice" role="note">
        <span className="demo-notice-icon">
          <Icon name="leaf" size={19} />
        </span>
        <div>
          <strong>Un assaggio di quello che verrà.</strong>
          <p>
            Feed demo con persone e post inventati. Accedi per pubblicare,
            interagire e leggere la tua timeline reale.
          </p>
        </div>
      </div>
      <section aria-label="Feed dimostrativo" className="timeline">
        {posts.map((post) => (
          <article className="post-card" key={post.handle}>
            <span className={`avatar ${post.color}`} aria-hidden="true">
              {post.initials}
            </span>
            <div className="post-content">
              <div className="post-meta">
                <strong>{post.name}</strong>
                <span>@{post.handle}</span>
                <span>· {post.time}</span>
                <span className="post-demo">Demo</span>
              </div>
              <p className="post-text">{post.text}</p>
              {post.tag && (
                <span className="post-tag">
                  # {post.tag.replaceAll(" ", "")}
                </span>
              )}
              <div
                className="post-actions"
                aria-label="Interazioni dimostrative: accedi per partecipare"
              >
                {(["reply", "repeat", "heart", "share"] as const).map(
                  (name, index) => (
                    <button
                      key={name}
                      disabled
                      aria-label={`${["Rispondi", "Ripubblica", "Mi piace", "Condividi"][index]}: dimostrazione`}
                    >
                      <Icon name={name} size={18} />
                      <span>—</span>
                    </button>
                  ),
                )}
              </div>
            </div>
          </article>
        ))}
      </section>
      <section className="feed-end">
        <Icon name="leaf" size={30} />
        <h2>C’è spazio anche per te.</h2>
        <p>Il prossimo passo è conoscerci.</p>
        <Link href="/register" className="text-button">
          Crea il tuo account <Icon name="arrow" size={16} />
        </Link>
      </section>
    </>
  );
}
