"use client";
import { useState } from "react";
import Link from "next/link";
import { api, ApiError } from "@/lib/api";
import { useResource } from "@/lib/use-resource";
import { useSession } from "@/components/auth/session-provider";
import { SessionGate, PageError } from "@/components/profile/shared";
import { SettingsNav } from "./settings-nav";
import styles from "./safety.module.css";

type Report = {
  id: string;
  target_type: "post" | "user";
  target_user_id: string;
  target_post_id: string;
  handle: string;
  reason: string;
  details: string;
  status: string;
  decision_reason: string;
  evidence?: string;
  account_status?: string;
  post_hidden: boolean;
  created_at: string;
};
type Audit = {
  id: string;
  action: string;
  target_type: string;
  target_id: string;
  moderator_id: string;
  reason: string;
  created_at: string;
};
type Page<T> = { items: T[]; next_cursor?: string };
const statuses: Record<string, string> = {
  open: "Da esaminare",
  resolved: "Gestita",
  dismissed: "Archiviata",
};
const reasons: Record<string, string> = {
  spam: "Spam",
  harassment: "Molestie o abusi",
  illegal: "Contenuto ritenuto illecito",
  other: "Altro",
};
const actions: Record<string, string> = {
  dismiss: "Archivia senza restrizioni",
  hide_post: "Nascondi post",
  restore_post: "Ripristina post",
  suspend_user: "Sospendi account",
  restore_user: "Riattiva account",
  grant_moderator: "Assegna ruolo moderatore",
  revoke_moderator: "Revoca ruolo moderatore",
};
function DecisionForm({
  report,
  refresh,
}: {
  report: Report;
  refresh: () => Promise<void>;
}) {
  const [pending, setPending] = useState(false);
  const [error, setError] = useState("");
  const options =
    report.status === "open"
      ? [
          "dismiss",
          ...(report.target_type === "post" && !report.post_hidden
            ? ["hide_post"]
            : []),
          ...(report.account_status === "active" ? ["suspend_user"] : []),
        ]
      : [
          ...(report.target_type === "post" && report.post_hidden
            ? ["restore_post"]
            : []),
          ...(report.account_status === "suspended" ? ["restore_user"] : []),
        ];
  if (!options.length) return null;
  return (
    <form
      className={styles.form}
      aria-label="Decisione moderatore"
      onSubmit={async (e) => {
        e.preventDefault();
        const form = new FormData(e.currentTarget);
        setPending(true);
        setError("");
        try {
          await api(`/moderation/reports/${report.id}/decisions`, {
            body: { action: form.get("action"), reason: form.get("reason") },
          });
          await refresh();
        } catch (err) {
          setError(
            err instanceof ApiError
              ? Object.values(err.fields).join(" ") || err.message
              : "Decisione non salvata.",
          );
        } finally {
          setPending(false);
        }
      }}
    >
      <label>
        Azione
        <select name="action" aria-label="Azione" disabled={pending}>
          {options.map((a) => (
            <option key={a} value={a}>
              {actions[a]}
            </option>
          ))}
        </select>
      </label>
      <label>
        Motivazione della decisione
        <textarea
          name="reason"
          required
          minLength={10}
          maxLength={2000}
          rows={3}
          disabled={pending}
        />
      </label>
      <p className="muted">
        La decisione viene registrata nell’audit. La motivazione è visibile al
        segnalante: non includere dati privati.
      </p>
      <button className="button primary" disabled={pending}>
        {pending ? "Salvataggio…" : "Conferma decisione"}
      </button>
      {error && (
        <p role="alert" className="form-error">
          {error}
        </p>
      )}
    </form>
  );
}
export function ReportsView({ moderation = false }: { moderation?: boolean }) {
  const { user } = useSession();
  return (
    <SessionGate>
      <header className="feed-header">
        <div>
          <h1>{moderation ? "Moderazione" : "Le tue segnalazioni"}</h1>
          <p>
            {moderation
              ? "Decisioni motivate e registro delle azioni"
              : "Controlla la ricezione e l’esito"}
          </p>
        </div>
      </header>
      <SettingsNav />
      {moderation && user?.role !== "moderator" ? (
        <PageError message="Accesso riservato ai moderatori." />
      ) : (
        <ReportsContent key={user?.id} moderation={moderation} />
      )}
    </SessionGate>
  );
}
function ReportsContent({ moderation }: { moderation: boolean }) {
  const [status, setStatus] = useState(moderation ? "open" : "");
  const [audit, setAudit] = useState(false);
  return (
    <>
      <div className={styles.toolbar}>
        {moderation && (
          <button
            className="text-button"
            aria-pressed={audit}
            onClick={() => setAudit(!audit)}
          >
            {audit ? "Apri segnalazioni" : "Apri audit"}
          </button>
        )}
        {!audit && (
          <label>
            Stato
            <select
              aria-label="Stato"
              value={status}
              onChange={(e) => setStatus(e.target.value)}
            >
              <option value="">Tutte</option>
              {Object.entries(statuses).map(([s, name]) => (
                <option key={s} value={s}>
                  {name}
                </option>
              ))}
            </select>
          </label>
        )}
      </div>
      <ReportPages
        key={`${status}:${audit}`}
        path={
          audit
            ? "/moderation/audit"
            : `${moderation ? "/moderation" : ""}/reports?status=${status}`
        }
        moderation={moderation}
        audit={audit}
      />
    </>
  );
}
function ReportPages({
  path,
  moderation,
  audit,
}: {
  path: string;
  moderation: boolean;
  audit: boolean;
}) {
  const [cursor, setCursor] = useState("");
  const resource = useResource<{ data: Page<Report | Audit> }>(
    `${path}${path.includes("?") ? "&" : "?"}cursor=${encodeURIComponent(cursor)}`,
  );
  const page = resource.data?.data;
  return (
    <div className={styles.content}>
      {resource.error ? (
        <PageError
          message={resource.error}
          retry={() => void resource.refresh()}
        />
      ) : !page ? (
        <p role="status">Caricamento…</p>
      ) : (
        <>
          {!page.items.length && (
            <p>
              Nessuna {audit ? "azione registrata" : "segnalazione presente"}.
            </p>
          )}
          {page.items.map((item) =>
            audit ? (
              <article key={item.id} className={styles.card}>
                <h2>{actions[(item as Audit).action]}</h2>
                <p className={styles.text}>{(item as Audit).reason}</p>
                <p className={styles.meta}>
                  Oggetto: {(item as Audit).target_type} ·{" "}
                  {(item as Audit).target_id}
                </p>
                <p className={styles.meta}>
                  Operatore: {(item as Audit).moderator_id || "CLI locale"} ·{" "}
                  {new Date(item.created_at).toLocaleString("it-IT")}
                </p>
              </article>
            ) : (
              <ReportCard
                key={item.id}
                report={item as Report}
                moderation={moderation}
                refresh={resource.refresh}
              />
            ),
          )}
          {page.next_cursor && (
            <button
              className="button"
              onClick={() => setCursor(page.next_cursor!)}
            >
              Pagina successiva
            </button>
          )}
          {cursor && (
            <button className="text-button" onClick={() => setCursor("")}>
              Torna all’inizio
            </button>
          )}
        </>
      )}
    </div>
  );
}
function ReportCard({
  report,
  moderation,
  refresh,
}: {
  report: Report;
  moderation: boolean;
  refresh: () => Promise<void>;
}) {
  return (
    <article className={styles.card} aria-label={`Segnalazione ${report.id}`}>
      <span className={styles.badge}>{statuses[report.status]}</span>
      <h2>
        {reasons[report.reason]} · @{report.handle}
      </h2>
      <p className={styles.meta}>
        {report.target_type === "post" ? "Post" : "Profilo"} ·{" "}
        {new Date(report.created_at).toLocaleString("it-IT")}
      </p>
      <p className={styles.text}>{report.details}</p>
      {moderation && report.evidence && (
        <blockquote className={styles.evidence}>{report.evidence}</blockquote>
      )}
      {report.decision_reason && (
        <p className={styles.text}>
          <strong>Motivazione:</strong> {report.decision_reason}
        </p>
      )}
      {report.target_post_id && (
        <Link className="text-button" href={`/posts/${report.target_post_id}`}>
          Apri post, se disponibile
        </Link>
      )}
      {moderation && (
        <>
          <p className={styles.meta}>
            Account: {report.account_status} · Post{" "}
            {report.post_hidden ? "nascosto" : "non nascosto"}
          </p>
          <DecisionForm report={report} refresh={refresh} />
        </>
      )}
    </article>
  );
}
