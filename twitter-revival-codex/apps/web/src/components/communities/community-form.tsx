"use client";

import { useId, useState } from "react";
import { api, ApiError } from "@/lib/api";
import {
  communitiesChanged,
  type Community,
  type CommunityValues,
} from "@/lib/communities";
import { useFormFocus } from "@/lib/use-form-focus";
import styles from "./communities.module.css";

export function CommunityForm({
  initial,
  onDone,
  onCancel,
}: {
  initial?: Community;
  onDone: (community: Community) => void;
  onCancel: () => void;
}) {
  const id = useId();
  const [values, setValues] = useState<CommunityValues>({
    name: initial?.name || "",
    description: initial?.description || "",
    rules: initial?.rules || "",
  });
  const [pending, setPending] = useState(false);
  const [error, setError] = useState("");
  const [fields, setFields] = useState<Record<string, string>>({});
  const form = useFormFocus(pending, fields, error);
  async function submit(event: React.FormEvent) {
    event.preventDefault();
    if (pending) return;
    const normalized = Object.fromEntries(
      Object.entries(values).map(([key, value]) => [key, value.trim()]),
    ) as CommunityValues;
    const invalid: Record<string, string> = {};
    if (
      Array.from(normalized.name).length < 3 ||
      Array.from(normalized.name).length > 60
    )
      invalid.name = "Scegli un nome da 3 a 60 caratteri.";
    if (Array.from(normalized.description).length > 500)
      invalid.description =
        "La descrizione può contenere al massimo 500 caratteri.";
    if (Array.from(normalized.rules).length > 1000)
      invalid.rules = "Le regole possono contenere al massimo 1000 caratteri.";
    setError("");
    setFields(invalid);
    if (Object.keys(invalid).length) return;
    setPending(true);
    try {
      const result = await api<{ data: { community: Community } }>(
        initial ? `/communities/${initial.id}` : "/communities",
        { method: initial ? "PATCH" : "POST", body: normalized },
      );
      communitiesChanged();
      onDone(result.data.community);
    } catch (err) {
      setError(
        err instanceof Error ? err.message : "Salvataggio non riuscito.",
      );
      if (err instanceof ApiError) setFields(err.fields);
    } finally {
      setPending(false);
    }
  }
  return (
    <form
      ref={form}
      className={styles.form}
      onSubmit={submit}
      aria-busy={pending}
      aria-label={initial ? "Modifica community" : "Crea community"}
      noValidate
    >
      <p className="field-hint">
        La community sarà visibile alle persone della beta. I post saranno
        riservati ai membri; chiunque nella beta può iscriversi.
      </p>
      {(
        [
          ["name", "Nome della community", 60],
          ["description", "Descrizione", 500],
          ["rules", "Regole della community", 1000],
        ] as const
      ).map(([key, label, max]) => (
        <div className="form-field" key={key}>
          <label htmlFor={`${id}-${key}`}>
            {label}
            {key === "name" ? " (obbligatorio)" : " (facoltativo)"}
          </label>
          {key === "name" ? (
            <input
              id={`${id}-${key}`}
              value={values[key]}
              required
              disabled={pending}
              onChange={(event) =>
                setValues((previous) => ({
                  ...previous,
                  [key]: event.target.value,
                }))
              }
              aria-invalid={!!fields[key]}
              aria-describedby={`${id}-${key}-hint`}
            />
          ) : (
            <textarea
              id={`${id}-${key}`}
              value={values[key]}
              rows={key === "rules" ? 5 : 3}
              disabled={pending}
              onChange={(event) =>
                setValues((previous) => ({
                  ...previous,
                  [key]: event.target.value,
                }))
              }
              aria-invalid={!!fields[key]}
              aria-describedby={`${id}-${key}-hint`}
            />
          )}
          <p
            id={`${id}-${key}-hint`}
            className={fields[key] ? "error-text" : "field-hint"}
          >
            {fields[key] ||
              `${Array.from(values[key].trim()).length}/${max} caratteri Unicode`}
          </p>
        </div>
      ))}
      {error && (
        <p className="form-error" role="alert" tabIndex={-1}>
          {error}
        </p>
      )}
      <div className={styles.actions}>
        <button className="button primary" disabled={pending}>
          {pending
            ? "Salvataggio…"
            : initial
              ? "Salva community"
              : "Crea community"}
        </button>
        <button
          className="text-button"
          type="button"
          onClick={onCancel}
          disabled={pending}
        >
          Annulla
        </button>
      </div>
    </form>
  );
}
