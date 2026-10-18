import { randomBytes } from "node:crypto";
import { readFile, writeFile, mkdir, stat, chmod } from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";

export const projectRoot = fileURLToPath(new URL("../", import.meta.url));
const keys = [
  "DOMAIN",
  "ACME_EMAIL",
  "RELEASE_TAG",
  "PUBLIC_BETA_ENABLED",
  "POSTGRES_PASSWORD",
  "S3_ACCESS_KEY",
  "S3_SECRET_KEY",
  "METRICS_TOKEN",
  "SMTP_ADDR",
  "SMTP_FROM",
  "SMTP_USERNAME",
  "SMTP_PASSWORD",
];
export function parseConfig(source) {
  const result = {};
  for (const line of source.split(/\r?\n/)) {
    if (!line.trim() || line.trimStart().startsWith("#")) continue;
    const match = /^([A-Z][A-Z0-9_]*)=(.*)$/.exec(line);
    if (!match || !keys.includes(match[1]))
      throw new Error("Formato o chiave di configurazione non ammessi.");
    const [, key, raw] = match;
    if (Object.hasOwn(result, key))
      throw new Error(`Chiave duplicata: ${key}.`);
    let value = raw.trim();
    if (value.startsWith("'") && value.endsWith("'")) {
      value = value.slice(1, -1);
      if (value.includes("'"))
        throw new Error(`Apici interni non ammessi: ${key}.`);
    } else if (/["'$#`\\]/.test(value))
      throw new Error(`Usare un valore letterale o apici singoli: ${key}.`);
    result[key] = value;
  }
  return result;
}
export function publicDomain(value = "") {
  return (
    value.length <= 253 &&
    value === value.toLowerCase() &&
    /^(?:[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z]{2,63}$/.test(value) &&
    !/(?:^|\.)(?:localhost|local|invalid|test|example|example\.com|example\.net|example\.org)$/.test(
      value,
    )
  );
}
function email(value = "") {
  const parts = value.split("@");
  return (
    parts.length === 2 &&
    /^[a-zA-Z0-9.!#$%&'*+/=?^_`{|}~-]{1,64}$/.test(parts[0]) &&
    publicDomain(parts[1])
  );
}
export function validateConfig(config) {
  const errors = [];
  for (const key of keys)
    if (!config[key] || config[key].includes("SET_ME"))
      errors.push(`${key}: valore richiesto.`);
  if (!publicDomain(config.DOMAIN))
    errors.push(
      "DOMAIN: serve un dominio pubblico, senza protocollo o percorso.",
    );
  if (!email(config.ACME_EMAIL))
    errors.push("ACME_EMAIL: contatto certificati non valido.");
  if (!/^[a-z0-9][a-z0-9._-]{0,63}$/.test(config.RELEASE_TAG ?? ""))
    errors.push("RELEASE_TAG: identificatore non valido.");
  if (!["true", "false"].includes(config.PUBLIC_BETA_ENABLED))
    errors.push("PUBLIC_BETA_ENABLED: usare true o false.");
  for (const key of ["POSTGRES_PASSWORD", "S3_SECRET_KEY", "METRICS_TOKEN"])
    if (!/^[a-f0-9]{64}$/.test(config[key] ?? ""))
      errors.push(
        `${key}: usare 32 byte casuali in esadecimale (64 caratteri).`,
      );
  if (
    new Set([
      config.POSTGRES_PASSWORD,
      config.S3_SECRET_KEY,
      config.METRICS_TOKEN,
    ]).size !== 3
  )
    errors.push("I tre segreti devono essere distinti.");
  if (!/^[a-zA-Z0-9_-]{16,64}$/.test(config.S3_ACCESS_KEY ?? ""))
    errors.push("S3_ACCESS_KEY: identificatore non valido.");
  const smtp = /^([^:]+):(587|2525)$/.exec(config.SMTP_ADDR ?? "");
  if (!smtp || !publicDomain(smtp[1]))
    errors.push(
      "SMTP_ADDR: relay pubblico con STARTTLS su porta 587 o 2525; porta 465 non supportata.",
    );
  const from = /^Radura <([^<>]+)>$/.exec(config.SMTP_FROM ?? "");
  if (!from || !email(from[1]) || from[1].split("@")[1] !== config.DOMAIN)
    errors.push(
      "SMTP_FROM: usare Radura <no-reply@DOMINIO> con mittente verificato.",
    );
  for (const key of ["SMTP_USERNAME", "SMTP_PASSWORD"])
    if ((config[key] ?? "").length > 512 || /[\r\n\0]/.test(config[key] ?? ""))
      errors.push(`${key}: valore non valido.`);
  return errors;
}
async function readEnvironment(file) {
  const info = await stat(file);
  if (process.platform !== "win32" && info.mode & 0o077)
    throw new Error("Proteggere il file ambiente con chmod 600.");
  return parseConfig(await readFile(file, "utf8"));
}
export async function readDomainConfig(file) {
  const config = await readEnvironment(file);
  if (!publicDomain(config.DOMAIN) || !email(config.ACME_EMAIL))
    throw new Error("Configurare dominio pubblico ed email operatore validi.");
  if (config.PUBLIC_BETA_ENABLED !== "false")
    throw new Error("La fase solo dominio richiede PUBLIC_BETA_ENABLED=false.");
  return config;
}
export async function readConfig(file) {
  const config = await readEnvironment(file);
  const errors = validateConfig(config);
  if (errors.length) throw new Error(errors.join("\n"));
  return config;
}
export async function initialize(root, domain, contact) {
  if (!publicDomain(domain) || !email(contact))
    throw new Error("Indicare dominio pubblico e email operatore validi.");
  const random = () => randomBytes(32).toString("hex");
  const config = {
    DOMAIN: domain,
    ACME_EMAIL: contact,
    RELEASE_TAG: "beta-001",
    PUBLIC_BETA_ENABLED: "false",
    POSTGRES_PASSWORD: random(),
    S3_ACCESS_KEY: randomBytes(12).toString("hex"),
    S3_SECRET_KEY: random(),
    METRICS_TOKEN: random(),
    SMTP_ADDR: "smtp.example.invalid:587",
    SMTP_FROM: `Radura <no-reply@${domain}>`,
    SMTP_USERNAME: "SET_ME",
    SMTP_PASSWORD: "SET_ME",
  };
  await writeFile(
    path.join(root, ".env.production"),
    Object.entries(config)
      .map(([key, value]) => `${key}=${value}`)
      .join("\n") + "\n",
    { flag: "wx", mode: 0o600 },
  );
  return config;
}
export async function prepare(root, file = path.join(root, ".env.production")) {
  const config = await readConfig(file);
  const destination = path.join(root, ".local/production");
  await mkdir(destination, { recursive: true, mode: 0o700 });
  await chmod(destination, 0o700);
  // The directory protects host access. Compose bind-backed secrets must be
  // readable by Prometheus's non-root UID inside its container.
  const secretFile = path.join(destination, "metrics-token");
  await writeFile(secretFile, config.METRICS_TOKEN, { mode: 0o644 });
  await chmod(secretFile, 0o644);
  return config;
}
if (
  process.argv[1] &&
  path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)
) {
  try {
    const [action, ...args] = process.argv.slice(2);
    if (action === "init") {
      await initialize(projectRoot, args[0], args[1]);
      console.log(
        "Creato .env.production con segreti nuovi. Configurare SMTP prima dell'avvio completo; il solo dominio può partire in manutenzione. Nessun file esistente sovrascritto.",
      );
    } else if (["check", "check-domain", "prepare"].includes(action)) {
      const file = args[0]
        ? path.resolve(args[0])
        : path.join(projectRoot, ".env.production");
      const config =
        action === "prepare"
          ? await prepare(projectRoot, file)
          : action === "check-domain"
            ? await readDomainConfig(file)
            : await readConfig(file);
      console.log(
        `Configurazione server valida; modalità pubblica ${config.PUBLIC_BETA_ENABLED === "true" ? "abilitata" : "in manutenzione"}. Nessun collegamento esterno o deploy eseguito.`,
      );
    } else
      throw new Error(
        "Uso: init DOMINIO EMAIL | check-domain [FILE] | check [FILE] | prepare [FILE]",
      );
  } catch (error) {
    console.error(
      error.code === "EEXIST"
        ? "File .env.production già presente: conservarlo, nessun segreto rigenerato."
        : error.code
          ? "File di configurazione non disponibile o non scrivibile."
          : error.message,
    );
    process.exitCode = 1;
  }
}
