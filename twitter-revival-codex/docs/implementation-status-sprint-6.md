# Stato implementazione — Sprint 6

Data: **18 ottobre 2026**. Prodotto locale: **Radura**, progetto interno Twitter Revival. Beta a inviti implementata e verificata; per scelta esplicita dell’utente **soltanto locale, con HTTPS locale**. Pilotaggio umano e osservazione D7 ancora da svolgere.

## Stato iniziale e conservazione

- Letti integralmente AGENTS.md e PROJECT_SPEC.md. Progetto in `twitter-revival-codex`, root Git nella cartella superiore `twitter revival`.
- Git inizialmente pulito, ramo `main`. Nessun commit, push, reset o cambio ramo. Specifiche e prompt iniziale conservati.
- Stack sano con schema 5. Backup preliminare `.local/backups/before-sprint6.dump`, **135.452 byte**. Volumi DB/S3 esistenti preservati.
- `.env` conservato: aggiunta soltanto la variabile mancante METRICS_TOKEN, generata con CSPRNG e mai stampata. Token di monitor in file locale ignorato da Git; nessuna rotazione di credenziali DB/S3.
- Registro Sprint 5 archiviato in `docs/implementation-status-sprint-5.md`.

## Attività completate

- Migrazione **006** e rollback: coorti, inviti, audit inviti, membership/indicatori, trigger primi eventi e coda email persistente.
- Registrazione su invito: token CSPRNG hashato, email vincolata, scadenza, revoca e consumo atomico; capienza 30–50 con prenotazione posti. Login degli account precedenti preservato.
- Pannello moderatore `/moderation/beta`: configurazione, pausa/riapertura, creazione e revoca inviti, link mostrato una volta, ultimi 100, statistiche e code reali. Autorizzazione nel DB e Origin/CSRF; bootstrap CLI esplicito, nessun admin predefinito.
- Form reale collegato: codice invito, frammento URL rimosso subito, stati config/caricamento/errore/pausa. Shell aggiornata alla beta, layout desktop/tablet/mobile preservato.
- Indicatori pilota: email verificata, attivazione, primo follow/post/interazione, D7 su finestra completa. Account preesistenti e coorti `e2e-*`/`load-*` esclusi. Nessun dato pilota inventato.
- Export dei propri indicatori e cancellazione di membership/inviti/job personali. Retention inviti 30 giorni dopo scadenza/revoca, intenti email 24h. Informazioni privacy e inventario tecnico aggiornati.
- Remediation email: PostgreSQL, lease, ack condizionato al tentativo proprietario, retry/backoff e limite 32 pendenti. Test riavvio e perdita di lease; niente link in chiaro nei job.
- Metriche Prometheus protette, label limitate ai pattern di rotta; contatori, latenze, DB/goroutine/code. Monitor locale con retention 7 giorni e cinque allarmi; nessuna notifica esterna.
- Remediation monitor: annullamenti client classificati 499, scadenze 504; panic reali ancora contati 500. Dopo la correzione, regressione browser con **zero serie 5xx** e annullamenti separati.
- HTTPS Caddy locale con CA interna, cookie `__Host-` Secure/HttpOnly, origine HTTPS esatta. Corretto il matcher redirect per escludere l’healthcheck. Certificato pubblico esportato; nessuna modifica automatica alle autorità Windows.
- Carico sintetico riproducibile con 50 utenti, quote attive, soglie e cleanup tramite API. Workflow CI aggiornati nella root Git effettiva e nel template interno, con test delle regole monitor.

## File creati principali

```text
services/api/migrations/006_private_beta.{up,down}.sql
services/api/internal/beta/service.go
services/api/internal/monitoring/metrics.go
services/api/internal/auth/mail_queue.go
services/api/internal/httpapi/{beta.go,sprint6_integration_test.go}
apps/web/src/app/moderation/beta/page.tsx
apps/web/src/components/safety/beta-dashboard.tsx
apps/web/tests/{beta,https}.spec.ts
playwright.https.config.ts
infra/{https.compose.yaml,Caddyfile.local}
infra/monitoring/{prometheus.yaml,alerts.yaml,alerts.test.yaml}
scripts/{prepare-sprint6,start-local-beta}.ps1
scripts/load-test.mjs
docs/testing/load-sprint-6.json
docs/architecture/sprint-6.md
docs/operations/local-beta.md
docs/security/sprint-6-review.md
docs/reverse-engineering/sprint-6-matrix.md
docs/implementation-status-sprint-5.md
```

Aggiornati inoltre server/CLI/router, auth/repository/worker, privacy/retention, sqlc/modelli generati, runner migrazioni, form/shell/settings/CSS, helper E2E, test errori e rollback Sprint 5, Compose, `.env.example`, `.gitignore`, package scripts, README, inventario privacy e procedura backup. Nessun servizio cloud o funzione fuori roadmap.

## Nuovi endpoint funzionanti

| Metodo/percorso                               | Accesso/risultato                              |
| --------------------------------------------- | ---------------------------------------------- |
| GET `/api/v1/beta`                            | Pubblico: obbligo invito e stato ingressi      |
| GET `/api/v1/moderation/beta`                 | Moderatore: metriche pilota e operazioni       |
| PATCH `/api/v1/moderation/beta`               | Moderatore + Origin/CSRF: capienza/pausa → 204 |
| GET `/api/v1/moderation/beta/invites`         | Moderatore: ultimi 100, nessun token/hash      |
| POST `/api/v1/moderation/beta/invites`        | Moderatore + Origin/CSRF: emissione → 201      |
| DELETE `/api/v1/moderation/beta/invites/{id}` | Moderatore + Origin/CSRF: revoca → 204         |
| GET `/metrics` sulla API interna              | Bearer dedicato; dal gateway sito → 404        |

Auth, profili, social, post, feed, immagini, ricerca, notifiche, report/moderazione ed export/delete precedenti restano reali e coperti dalle regressioni. `GET /healthz` interno verifica DB/schema/S3.

## Test e risultati effettivi

| Verifica                                                           | Esito                                                                                                |
| ------------------------------------------------------------------ | ---------------------------------------------------------------------------------------------------- |
| Go format, sqlc generate, go vet, build Windows e Docker           | Passati                                                                                              |
| `go test -count=1 -json ./...` Windows                             | **45 test principali + 31 sottocasi passati, 0 falliti/saltati**                                     |
| Stessa suite Linux Docker con `go test -race -count=1 -json ./...` | **45 + 31 passati, 0 falliti/saltati, nessuna race rilevata**                                        |
| Prettier, TypeScript, ESLint                                       | Passati, compreso controllo finale della documentazione                                              |
| Build Next di produzione in Docker                                 | Passata, compresa `/moderation/beta`                                                                 |
| Playwright completo HTTP, API/PG/S3/SMTP reali                     | **42/42 passati**, tre viewport, ultima esecuzione 2,9 minuti                                        |
| Playwright HTTPS locale                                            | **3/3 passati**, tre viewport, 17,9 secondi                                                          |
| CA e hostname Node con `rejectUnauthorized: true`                  | Passati; cookie Secure/HttpOnly/SameSite/Path, redirect, persistenza, post, CSRF e revoca verificati |
| Compose, Caddy validate, stack HTTP/HTTPS                          | Passati dopo correzione matcher healthcheck                                                          |
| Promtool configurazione e cinque regole                            | Passati; tre scenari sintetici di allarme passati                                                    |
| Target Prometheus reale                                            | `radura` UP=1 verificato                                                                             |
| `npm audit --omit=dev`                                             | **0 vulnerabilità segnalate**; solo dipendenze npm di produzione                                     |
| Backup/restore schema 006 + S3                                     | Passato: digest identici, 21 tabelle, 2.008 righe, 19 immagini                                       |

Test nuovi: invito assente/email errata/scaduto/revocato/riusato; conflitto non consuma il codice; consumo concorrente unico; ultimo posto non sovraprenotato; ruoli e CSRF; eventi primi passi e D7 con orologio DB modificato solo nello schema isolato; esclusione test; rollback preserva identità/sessioni; export/delete nuovi dati; SMTP fallito con ripresa da nuovo worker; ack obsoleto non elimina una lease nuova; metriche senza valori privati e panic effettivamente contati.

Gli screenshot del pannello desktop/mobile sono stati ispezionati. Screenshot, trace, export di test, CA e log grezzi restano locali/ignorati. Le prove backend usano schemi isolati in `radura_test` e bucket `radura-tests`, senza rollback o azzeramento del database attivo.

## Carico locale misurato

Rapporto aggregato: `docs/testing/load-sprint-6.json`, UTC 2026-10-18 17:54:31. Prova HTTP loopback, non contemporanea agli E2E o al backup; setup/cleanup fuori dai percentili.

- **50 utenti sintetici concorrenti**, circa una richiesta per utente al secondo; feed/dettaglio/profilo/like.
- **1.489 richieste in 30,93s**, **48,15 richieste/s**.
- Latenza client **p50 26,43ms · p95 48,33ms · p99 61,75ms**.
- **1.489 risposte 200, zero 429/5xx/errori rete**. Soglie dichiarate rispettate.
- **50/50 account creati eliminati via API** con la propria password; nessun account preesistente cancellato. Coorte di test esclusa dalle metriche pilota; tombstone secondo la politica esistente.
- È una misura di carico moderato di questa macchina, non un benchmark di saturazione né una garanzia per 50 persone su Internet.

## Limiti e attività non svolte

- Nessun dominio/server pubblico: solo loopback. Persone su altri dispositivi non possono accedere in questa configurazione.
- Nessun reclutamento/invio inviti a terzi; piloti umani e retention D7 reale **ancora da osservare**. Il polling autenticato può contribuire al ritorno D7: indicatore tecnico, non prova di attenzione umana.
- CA Windows non installata: il browser può mostrare un avviso finché il gestore non importa manualmente il certificato pubblico. Chromium ignora soltanto la CA privata nella configurazione TLS locale; Node verifica catena e hostname realmente.
- Mailpit è una casella privata locale, non un SMTP di consegna esterno. At-least-once può duplicare un invio dopo crash; vale soltanto il link più recente. Quote conservativamente condivise dai browser dietro Next.
- Allarmi visibili in Prometheus, senza recapito esterno. Nessuna simulazione di carico distribuito, pentest indipendente o promessa di capacità produttiva.
- CI aggiornata ma pipeline remota **non eseguita**: nessun push autorizzato. Race detector eseguito realmente in Linux locale.
- Restano revisione giuridica, contatti/canali pubblici, ripristino assistito con cancellazioni successive al backup e altri limiti operativi già documentati. Nessun claim di conformità legale.
- Nessun commit/deploy esterno, DM, ActivityPub, microservizi o acquisto.

## Avvio e prossimo passo

```powershell
# Dalla directory twitter-revival-codex, Docker Desktop attivo:
./scripts/start-local-beta.ps1
# Ricompilare quando cambia il codice:
./scripts/start-local-beta.ps1 -Build
```

Sito **https://localhost:3443**, monitor **http://localhost:9090**, email locale **http://localhost:8025**. Comandi completi per invito iniziale, ruolo moderatore, CA, test e stop conservando i dati nel README e in `docs/operations/local-beta.md`.

Prossimo passo operativo: scegliere l’account moderatore del gestore, accedere su HTTPS e creare il primo invito personale. Pilotaggio umano ed eventuale accesso esterno richiedono attività successive esplicite; nessuna feature futura aggiunta automaticamente.

Stato finale verificato: sette servizi sani su loopback, schema 6, Prometheus UP=1, HTTPS con CA verificata anche dopo il riavvio del backup. Coorte pilot: 0 membri e 0 inviti attivi; coda email vuota. Backup verificato in .local/backups/sprint5/20261018T180137Z_3797165f, con rimozione dei soli container/volumi temporanei di restore.
