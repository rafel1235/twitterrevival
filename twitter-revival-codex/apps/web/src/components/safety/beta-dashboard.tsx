"use client";
import { useState } from "react";
import { api, ApiError } from "@/lib/api";
import { useResource } from "@/lib/use-resource";
import { useSession } from "@/components/auth/session-provider";
import { SessionGate, PageError } from "@/components/profile/shared";
import { SettingsNav } from "./settings-nav";
import styles from "./safety.module.css";
type Stats = {
  capacity: number;
  enrollment_open: boolean;
  members: number;
  reserved: number;
  verified: number;
  activated: number;
  first_follow: number;
  first_post: number;
  first_interaction: number;
  d7_eligible: number;
  d7_returned: number;
  d7_rate: number | null;
};
type Operations = {
  uptime_seconds: number;
  requests: number;
  errors: number;
  mail_pending: number;
  mail_failed: number;
  media_pending: number;
};
type Invite = { id: string; email: string; status: string; expires_at: string };
export function BetaDashboard() {
  const { user } = useSession();
  return (
    <SessionGate>
      <header className="feed-header">
        <div>
          <h1>Beta privata</h1>
          <p>Inviti, primi passi e stato del servizio</p>
        </div>
      </header>
      <SettingsNav />
      {user?.role === "moderator" ? (
        <Dashboard />
      ) : (
        <PageError message="Accesso riservato ai moderatori." />
      )}
    </SessionGate>
  );
}
function Dashboard() {
  const resource = useResource<{
    data: { beta: Stats; operations: Operations };
  }>("/moderation/beta");
  const invites = useResource<{ data: { items: Invite[] } }>(
    "/moderation/beta/invites",
  );
  const [pending, setPending] = useState(false);
  const [error, setError] = useState("");
  const [link, setLink] = useState("");
  async function refresh() {
    await Promise.all([resource.refresh(), invites.refresh()]);
  }
  async function run(action: () => Promise<unknown>) {
    setPending(true);
    setError("");
    try {
      await action();
      await refresh();
    } catch (e) {
      setError(
        e instanceof ApiError
          ? Object.values(e.fields).join(" ") || e.message
          : "Operazione non riuscita.",
      );
    } finally {
      setPending(false);
    }
  }
  const s = resource.data?.data.beta;
  const ops = resource.data?.data.operations;
  if (resource.error)
    return (
      <PageError
        message={resource.error}
        retry={() => void resource.refresh()}
      />
    );
  if (!s)
    return (
      <p className={styles.content} role="status">
        Caricamento beta…
      </p>
    );
  return (
    <div className={styles.content}>
      <p>
        Coorte piloti: {s.members}/{s.capacity} membri · {s.reserved} inviti
        attivi. {s.enrollment_open ? "Ingressi aperti." : "Ingressi in pausa."}{" "}
        Gli account precedenti e le coorti di test non entrano nelle metriche.
      </p>
      <div className={styles.metrics} aria-label="Metriche piloti">
        {[
          ["Email verificate", s.verified],
          ["Attivati", s.activated],
          ["Primo follow", s.first_follow],
          ["Primo post", s.first_post],
          ["Prima interazione", s.first_interaction],
          [
            "Retention D7",
            s.d7_rate === null
              ? "Non ancora misurabile"
              : `${Math.round(s.d7_rate * 100)}% (${s.d7_returned}/${s.d7_eligible})`,
          ],
        ].map(([name, value]) => (
          <div key={name}>
            <strong>{value}</strong>
            <span>{name}</span>
          </div>
        ))}
      </div>
      <p className="muted">
        Attivazione: email verificata, primo follow e primo post. D7: attività
        autenticata fra 7 e 8 giorni dall’ingresso; percentuale calcolata solo
        dopo la fine di quella finestra. Nessuna previsione sui risultati
        futuri.
      </p>
      <form
        className={styles.form}
        aria-label="Configura beta"
        onSubmit={(e) => {
          e.preventDefault();
          const form = new FormData(e.currentTarget);
          void run(() =>
            api("/moderation/beta", {
              method: "PATCH",
              body: {
                capacity: Number(form.get("capacity")),
                enrollment_open: s.enrollment_open,
              },
            }),
          );
        }}
      >
        <label>
          Posti piloti (30–50)
          <input
            type="number"
            name="capacity"
            min={30}
            max={50}
            defaultValue={s.capacity}
            key={s.capacity}
            disabled={pending}
          />
        </label>
        <button className="button" disabled={pending}>
          Aggiorna posti
        </button>
      </form>
      <button
        className="text-button"
        disabled={pending}
        onClick={() =>
          void run(() =>
            api("/moderation/beta", {
              method: "PATCH",
              body: {
                capacity: s.capacity,
                enrollment_open: !s.enrollment_open,
              },
            }),
          )
        }
      >
        {s.enrollment_open
          ? "Metti in pausa gli ingressi"
          : "Riapri gli ingressi"}
      </button>
      <section className={styles.card}>
        <h2>Invita una persona</h2>
        <p>
          Il collegamento viene mostrato una sola volta. Condividilo
          privatamente con la persona indicata; il servizio non invia
          automaticamente questo invito.
        </p>
        <form
          className={styles.form}
          aria-label="Crea invito"
          onSubmit={(e) => {
            e.preventDefault();
            const form = new FormData(e.currentTarget);
            setLink("");
            void run(async () => {
              const result = await api<{ data: { token: string } }>(
                "/moderation/beta/invites",
                {
                  body: {
                    email: form.get("email"),
                    days: Number(form.get("days")),
                  },
                },
              );
              setLink(
                `${window.location.origin}/register#invite=${result.data.token}`,
              );
            });
          }}
        >
          <label>
            Email del destinatario
            <input
              type="email"
              name="email"
              required
              maxLength={254}
              autoComplete="off"
              disabled={pending}
            />
          </label>
          <label>
            Scadenza in giorni
            <input
              type="number"
              name="days"
              defaultValue={7}
              min={1}
              max={14}
              required
              disabled={pending}
            />
          </label>
          <button className="button primary" disabled={pending}>
            Crea invito
          </button>
        </form>
        {link && (
          <div className={styles.form} role="status">
            <label>
              Collegamento privato
              <input
                readOnly
                value={link}
                onFocus={(e) => e.currentTarget.select()}
              />
            </label>
            <button className="text-button" onClick={() => setLink("")}>
              Nascondi collegamento
            </button>
          </div>
        )}
      </section>
      <section className={styles.card}>
        <h2>Ultimi 100 inviti piloti</h2>
        {invites.error ? (
          <PageError
            message={invites.error}
            retry={() => void invites.refresh()}
          />
        ) : !invites.data ? (
          <p role="status">Caricamento…</p>
        ) : !invites.data.data.items.length ? (
          <p>Nessun invito creato.</p>
        ) : (
          invites.data.data.items.map((i) => (
            <article key={i.id} className={styles.invite}>
              <strong>{i.email}</strong>
              <p className="muted">
                {
                  (
                    {
                      available: "Disponibile",
                      used: "Usato",
                      revoked: "Revocato",
                      expired: "Scaduto",
                    } as Record<string, string>
                  )[i.status]
                }{" "}
                · Scade {new Date(i.expires_at).toLocaleString("it-IT")}
              </p>
              {i.status === "available" && (
                <button
                  className="text-button"
                  disabled={pending}
                  onClick={() =>
                    void run(() =>
                      api(`/moderation/beta/invites/${i.id}`, {
                        method: "DELETE",
                        body: {},
                      }),
                    )
                  }
                >
                  Revoca invito
                </button>
              )}
            </article>
          ))
        )}
      </section>
      {ops && (
        <section className={styles.card}>
          <h2>Stato operativo</h2>
          <p>
            {ops.requests} richieste · {ops.errors} errori server ·{" "}
            {ops.mail_pending} email in coda · {ops.mail_failed} email da
            verificare · {ops.media_pending} immagini in attesa di eliminazione.
          </p>
          <p className="muted">
            Contatori HTTP dall’avvio dell’istanza; le code sono persistenti.
          </p>
        </section>
      )}
      <button
        className="text-button"
        disabled={pending}
        onClick={() => void refresh()}
      >
        Aggiorna dati
      </button>
      {error && (
        <p role="alert" className="form-error">
          {error}
        </p>
      )}
    </div>
  );
}
