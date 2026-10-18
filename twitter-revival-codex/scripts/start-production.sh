#!/usr/bin/env bash
# Execute on the owned Linux VPS after its purchase and deployment approval.
set -euo pipefail
mode="${1:-}"
if [[ "$mode" != "" && "$mode" != --domain-only ]]; then
  printf '%s\n' 'Uso: start-production.sh [--domain-only]' >&2
  exit 1
fi
project_root="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
cd -- "$project_root"
if [[ "$(uname -s)" != Linux ]]; then
  printf '%s\n' 'Eseguire questo comando sul VPS Linux, non sul computer Windows.' >&2
  exit 1
fi
if [[ ! -f .env.production ]]; then
  printf '%s\n' 'Generare prima .env.production con dominio ed email operatore.' >&2
  exit 1
fi
docker info >/dev/null
# No eval/source of the environment file, no credentials in command arguments.
check_command=prepare
if [[ "$mode" == --domain-only ]]; then check_command=check-domain; fi
docker run --rm --network none --user "$(id -u):$(id -g)" \
  -v "$project_root:/workspace" -w /workspace node:24-alpine \
  node scripts/production-config.mjs "$check_command"
# The production file is authoritative; exported development variables must
# not silently replace its credentials or application origin.
unset DOMAIN ACME_EMAIL RELEASE_TAG PUBLIC_BETA_ENABLED POSTGRES_PASSWORD \
  S3_ACCESS_KEY S3_SECRET_KEY METRICS_TOKEN SMTP_ADDR SMTP_FROM SMTP_USERNAME SMTP_PASSWORD
compose=(docker compose --project-name radura-production --env-file .env.production -f compose.production.yaml)
"${compose[@]}" config --quiet
if [[ "$mode" == --domain-only ]]; then
  "${compose[@]}" up -d --no-deps --wait gateway
  "${compose[@]}" exec -T gateway caddy validate --config /etc/caddy/Caddyfile
  printf '%s\n' 'Solo dominio/HTTPS avviati in manutenzione; questo comando non avvia app e database.'
  exit 0
fi
"${compose[@]}" build api migrate web
"${compose[@]}" up -d --wait
"${compose[@]}" exec -T gateway caddy validate --config /etc/caddy/Caddyfile
printf '%s\n' 'Servizi avviati. Verificare DNS e certificato pubblico prima di aprire la beta.'
