# Stato implementazione · 18 ottobre 2026

**Sprint 3 completato e verificato localmente: post, risposte, like, repost, citazioni e timeline cronologica con PostgreSQL reale.** Lo stack aggiornato è attivo in Docker. Gli sprint precedenti restano funzionanti; resoconti conservati in [Sprint 1](implementation-status-sprint-1.md) e [Sprint 2](implementation-status-sprint-2.md). Nessuna funzione Sprint 4–6 implementata.

## Stato iniziale e preservazione

Letti interamente `AGENTS.md` e `PROJECT_SPEC.md`; lasciati invariati insieme a `CODEX_START_PROMPT.md`. Git iniziale pulito sul ramo `main`, nella cartella contenitore `twitter revival`. Nessuna modifica a indice, branch, cronologia o configurazione Git; nessun commit, push o deploy.

Prima della migrazione è stato salvato un backup PostgreSQL custom in `.local/backups/before-sprint3.dump` (**27.939 byte**). Database, volume Compose, `.env`, account, sessioni e grafo sociale preesistenti sono stati conservati. I test Go distruttivi usano schemi isolati nel database separato `radura_test`. I test browser creano account e contenuti di prova nel database locale del sito; non sono dati di persone reali. Segreti, backup, tool e artifact restano esclusi da Git.

## File e attività completate

| Area                | File principali                                                                                                 | Risultato                                                                                           |
| ------------------- | --------------------------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------- |
| Database            | `services/api/migrations/003_posts.{up,down}.sql`, `migrate.go`, `sqlc.yaml`                                    | posts/likes/reposts, vincoli, FK, indici, view di visibilità; schema versione 3                     |
| Query               | `services/api/db/queries/posts.sql`, `internal/db/posts.sql.go`, `internal/db/models.go`                        | SQL parametrizzato, generazione sqlc eseguita e usata dai repository                                |
| Modulo post         | `services/api/internal/posts/{service,repository,service_test}.go`                                              | Validazione Unicode, CRUD, interazioni idempotenti, feed/risposte/profilo, incorporamenti filtrati  |
| HTTP                | `services/api/internal/httpapi/{posts,server}.go`, `cmd/server/main.go`                                         | Route reali, autorizzazioni, origin/CSRF, rate limit, JSON coerente e status appropriati            |
| Integrazione        | `services/api/internal/httpapi/sprint3_integration_test.go`, fixture Sprint 2 aggiornata                        | Vincoli, CRUD, cursori, blocchi, sospensioni, concorrenza, upgrade/rollback                         |
| UI                  | `apps/web/src/components/posts/`, `src/lib/posts.ts`, `src/app/posts/[id]/page.tsx`                             | Composer, card, feed, dettaglio conversazione, risposte e citazioni; loading/empty/error/retry      |
| Integrazione UI     | `components/{timeline/timeline,profile/profile-view,layout/app-shell}.tsx`                                      | Home autenticata e profili con contenuti reali; demo solo anonima                                   |
| Browser             | `apps/web/tests/posts.spec.ts`, `helpers.ts`, test auth/identità aggiornati                                     | Flussi reali nelle tre viewport, helper che rispettano 429/Retry-After senza disattivare il limiter |
| CI e documentazione | Workflow interno e `../.github/workflows/radura-ci.yml`, README, architettura/matrice Sprint 3, questo registro | Pipeline estesa alla suite completa, contratti API e politiche documentati                          |

Non sono state aggiunte dipendenze npm/Go o nuovi servizi. Restano Next.js App Router/React/TypeScript, monolite Go/Chi/pgx/sqlc, PostgreSQL e Mailpit locale. Nessun asset proprietario, scraping o affiliazione suggerita; brand originale Radura.

## Endpoint funzionanti aggiunti

- `POST /api/v1/posts` → 201 con card e Location; `{body, reply_to_id?, quote_of_id?}`.
- `GET /api/v1/posts/{id}` → 200 con card visibile.
- `PATCH /api/v1/posts/{id}` → 200, solo testo del proprio post.
- `DELETE /api/v1/posts/{id}` → 204, soft delete ripetibile dal proprietario.
- `POST` / `DELETE /api/v1/posts/{id}/likes` → 200 con card aggiornata, idempotenti.
- `POST` / `DELETE /api/v1/posts/{id}/reposts` → 200 con card aggiornata, idempotenti.
- `GET /api/v1/posts/{id}/replies` → risposte dirette paginate.
- `GET /api/v1/feed/home` → post propri e dei seguiti più repost.
- `GET /api/v1/users/{id}/posts` → post e repost del profilo visibile.

Sessione obbligatoria anche in lettura; 401 anonimo, 422 input/UUID/cursore non valido, 403 modifica/eliminazione di un post altrui visibile, 404 assente/cancellato/bloccato/inattivo. Mutazioni JSON con origin esatto, Fetch Metadata e header CSRF; massimo 8 KiB. Quota separata 90 scritture post/minuto per peer, con 429/Retry-After; protezioni auth/social/email precedenti mantenute.

## Comportamenti e scelte verificate

Testo da 1 a 280 code point Unicode dopo trim coerente client/server, incluse differenze NEL/BOM. Emoji e accenti combinati contano per code point, non unità UTF-16. Nessun HTML interpretato. Modifica solo del testo proprio, indicatore “Modificato”, nessuno spostamento nel feed; riferimenti immutabili. Risposta e citazione sono alternative e richiedono testo proprio.

Originali e repost sono eventi distinti ordinati per data e UUID discendenti, senza ranking. Cursore Base64URL, limite default 20/max 50; card e riferimenti di un solo livello caricati in batch con snapshot coerente per richiesta. Blocchi e account inattivi filtrano autore, reposter, contatori e incorporamenti; un originale non disponibile è oscurato senza rivelarne nome/testo. Le risposte/citazioni di altri autori visibili restano contenuti indipendenti. Mutazioni serializzate con lo stesso lock per coppia del grafo sociale, poi lock del post e nuovo controllo di visibilità.

## Verifiche realmente eseguite

| Verifica                                                                                | Esito effettivo                                                                                                                                              |
| --------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `sqlc generate`, `gofmt`                                                                | PASS; generazione tipizzata e formattazione Go                                                                                                               |
| `go vet ./...`                                                                          | PASS                                                                                                                                                         |
| `TEST_DATABASE_URL=… go test -count=1 -v ./...`                                         | **22 test principali PASS + 22 sotto-casi (9 Origin/CSRF, 13 Unicode), nessuno skip**, PostgreSQL reale                                                      |
| `go build -o … ./cmd/server`                                                            | PASS, binario locale; compilazione API riuscita anche in Docker                                                                                              |
| `npm run format:check`, `npm run lint`, `npm run typecheck`                             | PASS, zero warning/errori ESLint                                                                                                                             |
| `npm run build`                                                                         | PASS nativo; build finale aggiornato riuscito in Docker con route dinamica `/posts/[id]`                                                                     |
| `docker compose config --quiet`                                                         | PASS                                                                                                                                                         |
| `docker compose up --build -d --wait --wait-timeout 180`                                | PASS; **db, api, web e mailpit healthy**, migrate **Exited (0)**                                                                                             |
| Healthcheck e schema                                                                    | `/api -healthcheck` exit 0; versione applicativa PostgreSQL **3**                                                                                            |
| `PLAYWRIGHT_BASE_URL=http://localhost:3000 E2E_AUTH=1 E2E_MAILPIT=1 npm run test:smoke` | **24/24 PASS, nessuno skip, 1,0 minuto**; ultimo test rispetta il Retry-After della API                                                                      |
| UI                                                                                      | Desktop 1440×900, tablet 768×1024, mobile 390×844; screenshot conversazione/timeline, assenza di overflow verificata, desktop/mobile ispezionati visivamente |

I nuovi test Go coprono pubblicazione/modifica/eliminazione, autorizzazioni e CSRF, 280/281 emoji, accenti combinati, NEL/BOM, FK/check SQL, like/repost ripetuti, contatori, risposte/citazioni e riferimenti oscurati. Coprono feed fuori grafo e repost consentiti, ordinamento a timestamp uguali, inserimenti fra pagine senza duplicati, blocchi in entrambi i versi, sospensioni, 8 like concorrenti, 8 round risposta/blocco e repost/cancellazione concorrenti. Il rollback della sola 003 conserva account/sessioni/profili; l'upgrade riparte correttamente.

Playwright verifica il ciclo completo via UI nelle tre viewport, persistenza dopo refresh, Unicode, HTML mostrato come testo, feed dopo follow, like/repost, risposte/citazioni, blocco e cancellazione. La viewport desktop verifica inoltre il caricamento oltre 20 eventi senza duplicati. Auth, profili, follow/blocchi e verifica/reset SMTP dello Sprint 2 restano nella suite. Nessun mock o intercettazione delle API. Report e screenshot in `playwright-report/` e `test-results/`, ignorati da Git.

Iterazioni risolte: reso esplicito il cast SQL dei riferimenti per sqlc; corretto un aggiornamento di stato segnalato dal lint nel caricamento della lista; uniformato il trim Unicode Go/JavaScript. Il primo run browser completo ha passato 23/24 test, incontrando la quota auth condivisa nell'ultimo login: gli helper ora attendono il Retry-After delle sole risposte 429, senza ridurre la protezione. Il run completo finale è passato. L'avvio Chromium/Docker richiede esecuzione fuori dal sandbox Windows; una prima prova browser ha restituito EPERM, poi è stata rieseguita con esito positivo.

Ambiente invariato dallo Sprint 2: Windows/PowerShell, Node 24.18.0/npm 11.16.0, Next 16.4.2/React 19.3.1/TypeScript 5.9.3, Go 1.27.2, sqlc 1.31.1, PostgreSQL 18.4, Docker Engine 29.8.2/Compose 5.6.0, Mailpit 1.31.6. Lockfile conservati.

## Limiti e prossimi passi

- Solo testo e visibilità public, con letture autenticate. Nessun upload, ricerca per parole chiave, notifiche, DM, federazione, moderazione o deploy.
- Nessun realtime: “Aggiorna” o refresh per azioni da altre sessioni. Una propria mutazione ricarica la prima pagina e non conserva le pagine aggiuntive aperte.
- Soft delete conserva il corpo nel DB e nei backup; niente revisioni, ripristino o policy definitiva di retention. Cancellazione/esportazione GDPR restano da implementare.
- I blocchi filtrano le righe storiche like/repost senza eliminarle: possono riapparire dopo sblocco. I follow rimossi non vengono ripristinati.
- Nessun load test; query di contatori e fan-out in lettura da misurare prima della beta. Quote per peer condivise dietro Next, non distribuite.
- Restano i limiti email dello Sprint 2: Mailpit solo locale, coda in memoria non durevole, verifica email non obbligatoria nell'anteprima. Retry durevoli e cleanup di token/sessioni ancora da completare.
- **CI GitHub Actions remota e race detector Linux non eseguiti.** Pipeline configurata; nessun push o pubblicazione.

Aprire <http://localhost:3000>, accedere con l'account locale, seguire un altro account e pubblicare. Casella email locale: <http://localhost:8025>. Servizi lasciati attivi. Dalla cartella del progetto: `docker compose up --build -d --wait` per ricompilare/avviare, `docker compose stop` per fermare conservando i dati; non sovrascrivere `.env`.

Il prossimo incremento è **Sprint 4: media, ricerca e notifiche**, da iniziare su richiesta. Setup ed elenco completo delle API nel README; politiche in [architettura Sprint 3](architecture/sprint-3.md), provenienza/stati UI nella [matrice Sprint 3](reverse-engineering/sprint-3-matrix.md).
