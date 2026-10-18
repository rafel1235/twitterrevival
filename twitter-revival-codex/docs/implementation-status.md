# Stato implementazione · 17 ottobre 2026

**Sprint 2 implementato e verificato contro PostgreSQL, API, frontend e SMTP locali reali.** Lo Sprint 1 resta funzionante. Il feed è ancora demo e il composer disabilitato: nessuna funzione dello Sprint 3 è stata anticipata. Il resoconto precedente è conservato in [implementation-status-sprint-1.md](implementation-status-sprint-1.md).

## Preservazione del lavoro

`AGENTS.md`, `PROJECT_SPEC.md` e `CODEX_START_PROMPT.md` sono stati letti e lasciati invariati. Il repository Git effettivo è nella cartella contenitore `twitter revival`; nessun cambiamento a indice, branch, cronologia o configurazione globale, nessun commit o push.

Prima dell'upgrade è stato creato un backup PostgreSQL custom in `.local/backups/before-sprint2.dump` (10.258 byte). La migrazione 002 conserva utenti, profili e sessioni preesistenti; volume Docker ed `.env` locale non sono stati azzerati o sovrascritti. I test usano il database separato `radura_test` e schemi isolati. `.env`, backup, tool, cache e artifact di test sono esclusi da Git.

## Attività completate e file principali

| Area                | Implementazione                                                                                                      | File principali                                                                                                                                                           |
| ------------------- | -------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Schema              | Avatar placeholder, follows, blocks, token account; FK, vincoli, indici e divieto di relazioni con sé stessi         | `services/api/migrations/002_identity_social.{up,down}.sql`                                                                                                               |
| Migrazioni          | Catalogo sequenziale, lock transazionale, rifiuto versioni sconosciute, rollback di una sola versione                | `services/api/migrations/migrate.go`, `services/api/sqlc.yaml`                                                                                                            |
| Profili e grafo     | Modifica del proprio profilo, follow/unfollow, block/unblock, contatori e liste a cursore filtrati per visibilità    | `services/api/internal/users/`, `internal/httpapi/users.go`, `db/queries/users.sql`, `internal/db/users.sql.go`                                                           |
| Sicurezza account   | Verifica email, recupero password, token hashati monouso, scadenza, cooldown, revoca di tutte le sessioni dopo reset | `services/api/internal/auth/accounts_{service,repository}.go`, `internal/httpapi/accounts.go`, `db/queries/account_tokens.sql`, `internal/db/account_tokens.sql.go`       |
| SMTP                | Worker interno limitato, timeout, STARTTLS e verifica certificato, invio locale a Mailpit                            | `services/api/internal/mailer/`, `cmd/server/main.go`, `compose.yaml`, `.env.example`                                                                                     |
| Frontend            | Profilo, editor, persone per handle esatto, connessioni, blocchi, verifica email e reset password collegati alle API | `apps/web/src/app/u/`, `app/settings/`, `app/people/`, `app/{verify-email,forgot-password,reset-password}/`, `components/profile/`, `components/auth/account-actions.tsx` |
| Sessione e UI       | Aggiornamento nome/avatar, gestione sessione scaduta e risposte obsolete dopo logout/reset, layout responsive        | `components/auth/session-provider.tsx`, `components/layout/app-shell.tsx`, `src/lib/{api,profile,use-resource}.ts`, `src/app/globals.css`                                 |
| Test                | Integrazione PostgreSQL, concorrenza, SMTP, validazione, browser reali nelle tre viewport                            | `services/api/internal/httpapi/sprint2_integration_test.go`, `internal/users/service_test.go`, `internal/mailer/smtp_test.go`, `apps/web/tests/identity.spec.ts`          |
| CI e documentazione | Workflow con PostgreSQL e Mailpit; setup, contratti API, politiche e provenienza UI                                  | `.github/workflows/ci.yml`, `README.md`, `docs/architecture/sprint-2.md`, `docs/reverse-engineering/sprint-2-matrix.md`                                                   |

I percorsi abbreviati nella tabella sono relativi alla rispettiva applicazione. Anche il codice sqlc dei modelli e dell'autenticazione è stato rigenerato. Il workflow effettivamente scoperto da GitHub nel repository attuale è `../.github/workflows/radura-ci.yml`, con working directory e percorsi adattati alla sottocartella; il template interno resta utilizzabile quando il monorepo è pubblicato come root.

## Endpoint nuovi funzionanti

- `GET /api/v1/users/{handle}` e `PATCH /api/v1/users/me`.
- `POST` / `DELETE /api/v1/users/{id}/follow`.
- `POST` / `DELETE /api/v1/users/{id}/block`.
- `GET /api/v1/users/{id}/followers`, `GET /api/v1/users/{id}/following`, `GET /api/v1/users/me/blocks`.
- `POST /api/v1/auth/verification/request` e `POST /api/v1/auth/verification/confirm`.
- `POST /api/v1/auth/password/forgot` e `POST /api/v1/auth/password/reset`.

Restano verificati `/healthz`, registrazione, login, logout e `/auth/me`. Profili ed elenchi richiedono una sessione. Il DTO pubblico non contiene email, password o token; l'attore delle mutazioni deriva sempre dal cookie. Nome massimo 50 code point, bio 160, località 100; URL solo HTTP/HTTPS senza credenziali, avatar a iniziali in quattro palette originali. Handle/email e upload non rientrano in questo incremento.

Un blocco elimina entrambi i follow nella stessa transazione e impedisce di ricrearli. Profilo ed elenchi diventano indisponibili in entrambe le direzioni; lo sblocco rimuove solo il blocco posseduto e non ripristina follow. Un lock PostgreSQL per coppia serializza le mutazioni concorrenti. Le liste hanno paginazione stabile per timestamp/UUID e riapplicano i controlli di visibilità.

I token email/reset sono casuali a 32 byte, nel DB solo SHA-256, con tipo e uso singolo. Scadono rispettivamente dopo 24 ore e 1 ora. Il recupero risponde con lo stesso `202` per account esistenti e inesistenti; il percorso HTTP accoda senza consultare l'account. I GET dei link non consumano token. Il reset aggiorna Argon2id e revoca tutte le sessioni atomicamente; un login concorrente non può creare una nuova sessione usando un hash precedente al reset. Mutazioni protette da origin esatto, Fetch Metadata, header CSRF, validazione JSON e limiti di frequenza.

## Verifiche realmente eseguite

| Verifica                                                                                | Esito                                                                                                                                         |
| --------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------- |
| `sqlc generate` e `gofmt`                                                               | PASS; query generate usate realmente, nessun file Go da formattare                                                                            |
| `go vet ./...`                                                                          | PASS                                                                                                                                          |
| `TEST_DATABASE_URL=… go test -count=1 -v ./...`                                         | **PASS: 17 test principali e 9 sotto-casi Origin/CSRF; nessuno skip**, PostgreSQL reale                                                       |
| `go build`                                                                              | PASS; binario locale e immagine API compilati                                                                                                 |
| `npm run format:check`, `npm run lint`, `npm run typecheck`                             | PASS; zero warning/errori ESLint                                                                                                              |
| `npm run build`                                                                         | PASS; build Next.js e verifica TypeScript, route profili dinamiche incluse                                                                    |
| `docker compose config --quiet`                                                         | PASS                                                                                                                                          |
| `docker compose up --build -d --wait`                                                   | PASS; **db, api, web e mailpit healthy**, migrazione **Exited (0)**                                                                           |
| Schema e healthcheck                                                                    | Versione DB **2**; `/api -healthcheck` exit 0                                                                                                 |
| `PLAYWRIGHT_BASE_URL=http://localhost:3000 E2E_AUTH=1 E2E_MAILPIT=1 npm run test:smoke` | **18/18 PASS, nessuno skip, 32,0 secondi**, contro i container reali                                                                          |
| UI e form                                                                               | Desktop 1440×900, tablet 768×1024 e mobile 390×844; screenshot ispezionati, nessun overflow anche con nome di 50 caratteri e bio di 160 emoji |

I test verificano persistenza del profilo dopo reload, autorizzazione alla sola modifica propria, assenza di dati privati, URL non sicuri, idempotenza delle relazioni, cursori, blocco reciproco, account sospesi e 12 round di follow/block concorrenti. Verificano inoltre token hashati, scadenza, tipo errato, replay, consumo concorrente con un solo successo, recupero non enumerabile, revoca sessioni e rifiuto di login con hash obsoleto. Il downgrade della sola 002 e il successivo upgrade conservano utenti/sessioni; una versione futura sconosciuta viene rifiutata.

Playwright mantiene la regressione auth dello Sprint 1 e aggiunge profili, follow/unfollow, block/unblock, verifica email e recupero password via SMTP reale catturato da Mailpit. Nessuna intercettazione/mock delle API. Report e screenshot sono in `playwright-report/` e `test-results/`, ignorati da Git. Corretto durante la verifica il ritorno a capo dei nomi lunghi; la suite finale è stata ripetuta con gli input massimi.

Ambiente: Windows/PowerShell, Node 24.18.0, npm 11.16.0, Next 16.4.2, React 19.3.1, TypeScript 5.9.3, Go 1.27.2, Chi 5.5.0, pgx 5.11.0, sqlc 1.31.1, PostgreSQL 18.4, Docker Engine 29.8.2, Compose 5.6.0, Mailpit 1.31.6. Nessuna nuova dipendenza npm/Go; Go/sqlc portabili sotto `.local/` e PATH Docker impostato solo per i processi di verifica.

## Limiti e attività non eseguite

- **Nessun post reale o altra funzione Sprint 3–6.** Home etichettata demo, composer disabilitato. Ricerca disponibile soltanto per handle esatto.
- Mailpit cattura email soltanto in locale. Nessun invio Internet, account esterno, servizio a pagamento o deploy. Un relay reale richiederà configurazione SMTP/TLS.
- Coda email in memoria, massimo 32 richieste e un worker: non è durevole. Un riavvio può perdere richieste pendenti; richiedere un nuovo link dopo il cooldown di un minuto per utente/tipo. Retry durevoli e retention da completare prima della beta.
- La verifica email è disponibile ma non obbligatoria per login/follow nell'anteprima locale.
- Limiti per peer condivisi dietro Next: auth 30/minuto, mutazioni sociali 60/minuto, operazioni email 10/minuto. Nessun affidamento su forwarded IP; massimo quattro operazioni Argon2 concorrenti. Da rivedere prima dell'uso pubblico.
- Pulizia di sessioni e token consumati/scaduti da definire prima della beta. Nessun cambio handle/email o upload avatar.
- **GitHub Actions remota non eseguita.** Race detector configurato nella CI Linux, non eseguito nel run Windows. Nessun push, commit o pubblicazione.

## Uso locale e prossimo passo

Servizi lasciati attivi: applicazione <http://localhost:3000>, modifica profilo <http://localhost:3000/settings/profile>, persone <http://localhost:3000/people>, casella email di sviluppo <http://localhost:8025>. Creare un account o usare quello locale esistente; aprire in Mailpit il link di verifica o recupero, poi confermare dal form.

Dalla cartella con `compose.yaml`: `docker compose up -d --wait` per riavviare, `docker compose stop` per arrestare conservando i dati. Non sovrascrivere `.env`; setup completo e comandi test nel README. Il prossimo incremento è lo **Sprint 3**, da aprire su richiesta, partendo dal backend dei post e sostituendo solo allora la demo.
