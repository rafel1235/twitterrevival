# Stato implementazione — Sprint 5

Data: **18 ottobre 2026**. Prodotto locale: **Radura**, progetto interno Twitter Revival. Sprint 5 implementato; nessuna implementazione Sprint 6 o pubblicazione esterna.

## Stato iniziale e conservazione

- Letti integralmente AGENTS.md e PROJECT_SPEC.md; repository applicativo annidato in `twitter-revival-codex`, root Git nella cartella superiore `twitter revival`.
- Git inizialmente pulito, ramo `main`. Nessun commit, push, cambio ramo, reset o modifica dei file di specifica. `.env` e i segreti esistenti conservati.
- Stack precedente sano con schema 4. Backup preliminare PostgreSQL `.local/backups/before-sprint5.dump`, **105.347 byte**. Nessun azzeramento del database o dei volumi attivi.
- Registro precedente archiviato in `docs/implementation-status-sprint-4.md`.

## Incremento realizzato

- Migrazione **005** con rollback: ruoli, report, audit, oscuramento dei post, stato purge media e rate limit persistenti.
- Report reali su post/profili, idempotenza per report aperto, ricevuta e lista personale paginata. Blocchi dello Sprint 2 preservati e coperti dalle regressioni.
- Pannello moderatore con filtro/cursori, prove testuali, decisioni motivate e audit transazionale. Oscuramento/ripristino, sospensione/riattivazione e revoca sessioni/token. Ruoli esclusivamente tramite CLI locale tracciata; nessun amministratore predefinito.
- Visibilità uniforme in feed, ricerca, anteprime, notifiche e download. L'autore può eliminare anche il proprio post oscurato; un moderatore non ripristina un post eliminato.
- Quote PostgreSQL per account e auth peer; deduplica dei post per 10 minuti, serializzata. URL con slash finale usano la stessa quota. Nessuna fiducia in X-Forwarded-For.
- Log di errore con sole classi tecniche: nessun errore grezzo pgx/S3, path, query o credenziale.
- Export ZIP reale con `data.json` e immagini proprie, campi ammessi espliciti, password attuale e protezioni sessione/Origin/CSRF. Nessun hash/token/chiave S3 nel file.
- Cancellazione account transazionale: pulizia immediata di identità/profilo/testi e relazioni, revoca sessioni/token, tombstone per preservare risposte altrui, coda durevole di eliminazione immagini.
- Manutenzione all'avvio e ogni minuto: bozze 24h, singoli post eliminati 30 giorni, report chiusi/audit 180 giorni. Retry S3 persistenti; riconciliazione orfani oltre 24h via CLI.
- Backup completo DB + volume SeaweedFS e restore isolato verificato con digest di tutte le righe e immagini. Script locale, ACL Windows e pulizia controllata delle proprie copie scadute.
- Pagine responsive per dati account, report, moderazione e informazioni privacy dell'anteprima. Bozza privacy/DSA con fonti primarie, inventario dati e procedure da definire prima del lancio.

## File creati principali

```text
services/api/migrations/005_safety_privacy.{up,down}.sql
services/api/db/queries/safety.sql
services/api/internal/safety/guard.go
services/api/internal/moderation/service.go
services/api/internal/privacy/{service,maintenance}.go
services/api/internal/backup/manifest.go
services/api/internal/httpapi/{safety.go,sprint5_integration_test.go}
services/api/internal/db/safety.sql.go
apps/web/src/components/safety/{settings-nav,report-button,account-settings,reports-view}.tsx
apps/web/src/components/safety/safety.module.css
apps/web/src/app/{moderation,privacy}/page.tsx
apps/web/src/app/settings/{account,reports}/page.tsx
apps/web/tests/safety.spec.ts
infra/restore.compose.yaml
scripts/backup-verify.ps1
docs/architecture/sprint-5.md
docs/reverse-engineering/sprint-5-matrix.md
docs/security/sprint-5-review.md
docs/legal/privacy-dsa-review.md
docs/operations/backup-restore.md
docs/implementation-status-sprint-4.md
```

File aggiornati: server/CLI e router, auth DTO/query/repository, lock delle scritture social/post/media, storage S3, runner/sqlc e query generate, card/profilo/navigazione/impostazioni, client API, test rollback Sprint 3/4, README e `.env.example`. Script format delimitati alle cartelle sorgenti per evitare l'accesso ai backup privati. Workflow aggiornati sia nella root Git effettiva `../.github/workflows/radura-ci.yml` sia nel template interno.

## Nuovi endpoint funzionanti

| Endpoint                                         | Comportamento                                    |
| ------------------------------------------------ | ------------------------------------------------ |
| `POST /api/v1/reports`                           | Crea/riceve report aperto, 201 con ID            |
| `GET /api/v1/reports`                            | Solo report propri, filtro e cursore             |
| `GET /api/v1/moderation/reports`                 | Coda riservata ai moderatori                     |
| `POST /api/v1/moderation/reports/{id}/decisions` | Decisione motivata e audit atomici, 204          |
| `GET /api/v1/moderation/audit`                   | Registro riservato paginato                      |
| `POST /api/v1/account/export`                    | Password attuale → ZIP privato                   |
| `DELETE /api/v1/account`                         | Password + ELIMINA → cancellazione e revoca, 204 |

Tutti richiedono sessione. Mutazioni con JSON, origine esatta e header CSRF; 403 per ruolo/password insufficienti, 404 per target indisponibile, 409 per stato concorrente/spam, 422 per validazioni, 429 con Retry-After per quota. Endpoint Sprint 1–4 preservati.

## Verifiche eseguite ed esiti reali

| Verifica                                               | Esito                                                                                                                                                                                                                    |
| ------------------------------------------------------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `sqlc generate` e `gofmt`                              | Query generate e Go formattato                                                                                                                                                                                           |
| `go vet ./...`                                         | Pass                                                                                                                                                                                                                     |
| `go test -count=1 -json ./...` con PostgreSQL/S3 reali | **37 test principali + 31 sottocasi passati; 0 test saltati; 0 fallimenti**                                                                                                                                              |
| Build Go nativa e immagini API/migrate                 | Pass                                                                                                                                                                                                                     |
| `npm run format:check`, `typecheck`, `lint`            | Pass, nessun warning ESLint nel lint                                                                                                                                                                                     |
| Build Next nativa e Docker                             | Pass; nuove route incluse                                                                                                                                                                                                |
| `E2E_AUTH=1 E2E_MAILPIT=1 npm run test:smoke`          | **39/39 pass**, desktop/tablet/mobile, **1,7 minuti**                                                                                                                                                                    |
| Avvio Compose e migrazioni                             | DB/storage/mailpit/API/web sani; migrate exit 0; schema **5**                                                                                                                                                            |
| Backup/restore completo                                | **Pass: 16 tabelle, 1.117 righe, 13 immagini S3**, conteggi/digest identici; copia `.local/backups/sprint5/20261018T154252Z_3814de87`, ricevuta `verified.json`. Due prove complete riuscite, volumi temporanei rimossi. |
| Sintassi script PowerShell                             | Pass                                                                                                                                                                                                                     |
| Diff whitespace e specifiche                           | `git diff --check` pass; AGENTS/PROJECT_SPEC/CODEX_START_PROMPT invariati                                                                                                                                                |

Test backend nuovi: autorizzazione/CSRF e privacy dei report, due decisioni concorrenti con audit unico, ripristino/sospensione/revoca ruolo, filtri sui contenuti e media, ZIP con immagini e assenza di dati di autenticazione, password errata, erasure con risposte altrui preservate, guasto DELETE S3 e retry durevole, scadenza bozze e retention post, orphan cleanup con grazia, quota persistente fra istanze, deduplica concorrente, scritture già autenticate bloccate dalla cancellazione e log con sentinelle sensibili.

Playwright usa API/PG/S3/SMTP reali e crea esclusivamente account sintetici. Il ruolo moderatore di ciascun test è revocato; verificato **0 moderatori di test residui**. Screenshot nuovi account/moderazione prodotti nelle tre viewport; ispezionati anche visivamente su mobile e desktop. Report HTML in `playwright-report`, screenshot/export sintetici in `test-results`, output Go in `.local/go-test-sprint5.jsonl`; tutti ignorati da Git.

Problemi emersi e risolti: tre regressioni iniziali dei contratti/rollback e ordine di validazione media; nomi accessibili dei select corretti dopo timeout E2E; interruzione di una prima prova browser sovrapposta alla pausa backup, poi prove separate; format inizialmente impedito dall'ACL dei backup, corretto usando glob sulle sole sorgenti. La suite finale sopra è passata senza retry applicativi fittizi o limiti disabilitati. Gli helper rispettano Retry-After reale.

### Pulizia manuale autorizzata sui dati correnti

Il **18 ottobre 2026 alle 18:48 Europe/Rome**, dopo l'autorizzazione esplicita dell'utente, eseguiti entrambi i comandi sullo stack persistente corrente. La precedente richiesta era stata bloccata dalla revisione automatica; la successiva esecuzione è avvenuta con l'approvazione del gestore.

| Verifica                   | Esito reale                                                          |
| -------------------------- | -------------------------------------------------------------------- |
| `/api -maintenance`        | Exit 0; immagini eliminate **0**, eliminazioni media in attesa **0** |
| `/api -reconcile-media`    | Exit 0; oggetti S3 orfani eliminati **0**                            |
| `/api -healthcheck`        | Exit 0                                                               |
| Stato Compose dopo pulizia | DB, storage, Mailpit, API e web tutti healthy                        |
| Smoke HTTP dopo pulizia    | Home e `/privacy`: 200; `/api/v1/auth/me` senza sessione: 401 atteso |

I contatori dei comandi riguardano gli oggetti media; non quantificano la pulizia dei record tecnici scaduti nel database.

## Limiti e attività successive

- **Nessuna dichiarazione di conformità o prontezza pubblica.** Titolare, contatti, basi giuridiche, notice esterne, comunicazioni agli autori e ricorsi da definire con consulenza. Account sospesi richiedono assistenza esterna al login ordinario.
- Export automatico limitato a 100 MiB immagini/32 MiB JSON/10.000 righe per categoria, oltre limite 413 e gestione assistita da predisporre. Un crash può lasciare un ZIP temporaneo da rimuovere operativamente.
- Tombstone/UUID e parte di report/audit restano pseudonimi; i testi liberi richiedono minimizzazione e revisione. Report aperti richiedono esame umano.
- Backup privati locali, senza cifratura automatica o copia fuori macchina. Retention dei backup gestita al successivo comando riuscito; dump manuali esclusi. Manca un registro esterno durevole per riapplicare cancellazioni dopo un vero disaster recovery.
- Quote peer Next condivise, antispam essenziale, nessun CAPTCHA/MFA. Coda email non durevole e verifica email non obbligatoria nell'anteprima.
- CI aggiornata ma **non eseguita su GitHub**; race detector Linux configurato, non eseguito nella shell Windows. Nessun penetration test indipendente, load test o deploy effettuato.

Prossimo passo operativo: il gestore sceglie un proprio handle esistente e assegna il ruolo con `docker compose exec api /api -role-user HANDLE -role moderator -reason "Assegnazione approvata dal gestore locale."`, poi prova `/moderation`. Concordare la revisione dei limiti sopra prima di autorizzare lo Sprint 6. Il sito rimane su **http://localhost:3000**, Mailpit su **http://localhost:8025**.
