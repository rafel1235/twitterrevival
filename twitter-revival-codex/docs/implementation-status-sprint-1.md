# Stato implementazione · 17 ottobre 2026

**Sprint 1 implementato e verificato localmente, anche con Docker Compose, Next.js, API Go e PostgreSQL reali.** CI remota ancora da verificare nel relativo ambiente. Nessuna funzione degli Sprint 2–6 implementata.

## Stato iniziale e preservazione

La directory di lavoro iniziale conteneva la sottocartella `twitter-revival-codex`, con `AGENTS.md`, `PROJECT_SPEC.md` e `CODEX_START_PROMPT.md`. I tre documenti sono stati letti e lasciati invariati. Non era presente codice applicativo preesistente.

All’inizio Git individuava un repository padre nella home, ramo `master`; il progetto risultava non tracciato. Alla verifica successiva è presente un repository dedicato nella cartella contenitore `twitter revival`. L’agente non ha modificato configurazioni Git, indice, branch, cronologia o file fuori dal progetto e non ha creato commit. Le letture Git hanno usato un’eccezione `safe.directory` limitata al singolo comando, senza configurazione globale.

## File e attività completate

| Area                 | File principali creati                                                                                                                                                                       | Esito                                                                                                       |
| -------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------- |
| Monorepo             | `package.json`, `package-lock.json`, `.gitignore`, `.prettierignore`, `.dockerignore`, `.env.example`                                                                                        | npm workspaces, lockfile, esclusione segreti/cache/tool locali                                              |
| Infrastruttura       | `compose.yaml`, `services/api/Dockerfile`, `apps/web/Dockerfile`                                                                                                                             | DB persistente → migrazione → API sana → web; runtime non-root                                              |
| Database             | `services/api/migrations/{001_auth.up.sql,001_auth.down.sql,migrate.go}`                                                                                                                     | users/profiles/sessions, CITEXT, FK, unique/check, indici, versione e advisory lock; rollback testato       |
| Query                | `services/api/sqlc.yaml`, `db/queries/auth.sql`, `internal/db/{db.go,models.go,auth.sql.go}`                                                                                                 | Generazione sqlc 1.31.1 eseguita; query parametrizzate usate realmente dal repository                       |
| Backend              | `services/api/go.mod`, `go.sum`, `cmd/server/main.go`, `internal/auth/{password.go,service.go,repository.go}`, `internal/httpapi/{server.go,limiter.go}`                                     | Cinque endpoint reali, validazione, transazioni, sessioni, cookie, CSRF/origin, rate limit, errori JSON     |
| Test Go              | `internal/auth/password_test.go`, `internal/httpapi/{server_test.go,integration_test.go}`                                                                                                    | Unit e integrazione con PostgreSQL reale, nessun mock in produzione                                         |
| Frontend             | `apps/web/{package.json,tsconfig.json,next.config.ts,eslint.config.mjs,next-env.d.ts}`, `src/app/{layout.tsx,page.tsx,login/page.tsx,register/page.tsx,not-found.tsx,globals.css,theme.css}` | App Router, proxy same-origin, responsive, token, brand originale Radura                                    |
| Componenti           | `src/lib/api.ts`, `src/components/auth/{session-provider.tsx,auth-form.tsx}`, `layout/{app-shell.tsx,icon.tsx}`, `timeline/timeline.tsx`                                                     | Sessione verificata dal server, form collegati, errori/loading, demo esplicita e azioni future disabilitate |
| Test browser / avvio | `playwright.config.ts`, `apps/web/tests/smoke.spec.ts`, `scripts/start-web.mjs`                                                                                                              | Tre viewport, ciclo auth reale, avvio dal build standalone con asset statici                                |
| CI                   | `.github/workflows/ci.yml`                                                                                                                                                                   | PostgreSQL, format/lint/tipi/build, test Go con race detector, Playwright, artifact                         |
| Documentazione       | `README.md`, `docs/architecture/sprint-1.md`, `docs/reverse-engineering/sprint-1-matrix.md`, questo documento                                                                                | Setup, variabili, API, migrazioni, assunzioni, provenienza UI, esiti e limiti                               |

## Endpoint verificati

- `GET /healthz`: 200, controllo database/schema; 503 testato con dipendenza non sana.
- `POST /api/v1/auth/register`: 201 e sessione; 409 per handle/email case-insensitive duplicati; 422 per dati invalidi.
- `POST /api/v1/auth/login`: 200 e nuovo token; 401 per password errata, email ignota o utente sospeso.
- `POST /api/v1/auth/logout`: 204, cookie scaduto e revoca persistita; ripetibile.
- `GET /api/v1/auth/me`: 200 con sessione valida; 401 senza sessione, con sessione scaduta/revocata o account sospeso.

Parametri Argon2id verificati: 64 MiB, 3 iterazioni, 2 lane, sale 16 byte, chiave 32 byte. Password e token grezzo non vengono memorizzati. SHA-256 del token nel DB; cookie HttpOnly/SameSite=Lax/Path=/, Secure e prefisso `__Host-` in HTTPS. La configurazione rifiuta HTTP fuori dal loopback.

## Ambiente effettivo

Windows, PowerShell; Node **24.18.0**, npm **11.16.0**, Next **16.4.2**, React **19.3.1**, TypeScript **5.9.3**, ESLint **10.13.0**, Prettier **3.9.10**, Playwright **1.64.1** con Chromium. Go **1.27.2**, Chi **5.5.0**, pgx **5.11.0**, x/crypto **0.58.0**, sqlc **1.31.1**, PostgreSQL **18.4**.

Go, PostgreSQL e sqlc non erano nel PATH; sono stati preparati tool portabili sotto `.local/`, ignorata da Git. Go e sqlc sono stati scaricati dalle release ufficiali con SHA-256 verificato. PostgreSQL reale è stato avviato su `127.0.0.1:55432` tramite `embedded-postgres@18.4.0-beta.17`, usato esclusivamente come tool temporaneo: non è una dipendenza del prodotto. API avviata sul loopback 8080. Nessun servizio a pagamento, account esterno o deploy.

## Verifiche native eseguite ed esiti

| Verifica                                        | Esito reale                                                                                                                                               |
| ----------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Installazione npm e Go                          | PASS; `package-lock.json` e `go.sum` generati                                                                                                             |
| `sqlc generate`                                 | PASS; codice generato collegato al repository                                                                                                             |
| `gofmt`, `go vet ./...`                         | PASS                                                                                                                                                      |
| `TEST_DATABASE_URL=… go test -count=1 -v ./...` | PASS: 9 test principali e 9 sotto-casi Origin/CSRF, incluso il ciclo con PostgreSQL; nessuno skip nel run verificato                                      |
| `go build -o … ./cmd/server`                    | PASS; binario avviato realmente                                                                                                                           |
| Migrazioni                                      | PASS: up ripetuto, down ripetuto, nuovo up; vincoli/FK e rollback transazionale della registrazione verificati in schema isolato                          |
| API avviata / healthcheck                       | PASS: comando `-healthcheck` exit 0; `/me` anonimo 401                                                                                                    |
| `npm run lint`                                  | PASS, zero warning/errori dopo correzione effetto React                                                                                                   |
| `npm run typecheck`                             | PASS; anche la verifica TypeScript nel build è riuscita                                                                                                   |
| `npm run build`                                 | PASS: build webpack, generazione 5 pagine statiche; route `/`, `/login`, `/register`, not-found                                                           |
| `E2E_AUTH=1 npm run test:smoke`                 | PASS: **9/9, nessuno skip**, ultimo run 28,3 secondi. Server Go e PostgreSQL reali, nessuna intercettazione/mock delle API                                |
| UI desktop/tablet/mobile                        | PASS: 1440×900, 768×1024, 390×844; nessun overflow orizzontale, demo esplicita e composer disabilitato. Screenshot desktop/mobile ispezionati visivamente |

Il ciclo Playwright verifica registrazione, cookie HttpOnly/SameSite, refresh con sessione mantenuta, logout, 401 successivo, errore credenziali e nuovo login riuscito. Screenshot e report sono in `test-results/` e `playwright-report/` (ignorati da Git).

Controlli finali tutti PASS: `npm run format:check`, `npm run typecheck`, `npm run lint`, `node --check scripts/start-web.mjs`, `gofmt -l` senza output. I documenti iniziali sono esclusi dal formatter per preservarli. Lo stato Git elenca esclusivamente file del progetto, senza tool locali, cache, cookie jar o credenziali di prova.

Iterazioni corrette: un primo lint ha segnalato aggiornamenti sincroni nell’effetto della sessione; risolti usando callback della richiesta asincrona. Un primo build è stato impedito da un processo concorrente, poi rieseguito con successo. Il primo Playwright ha passato 6 test UI e fermato 3 test auth su un selettore `role=alert` ambiguo con l’annunciatore di Next; il selettore è stato circoscritto al form e il run completo successivo è passato. Lo script start usa ora il vero output standalone, evitando il warning di `next start`.

## Verifica Docker successiva all’installazione

Docker Engine **29.8.2**, Compose **5.6.0**, WSL **2.6.3**: motore Linux avviato su Windows. Creato `.env` locale con password CSPRNG esadecimale, senza stampare credenziali e senza modificare `.env.example`; esclusione Git verificata con `git check-ignore`.

Esiti reali del 17 ottobre 2026:

- `docker compose config --quiet`: PASS.
- `docker compose up --build -d --wait --wait-timeout 180`: PASS. Immagini scaricate, backend e frontend compilati nei container; nessuna modifica funzionale necessaria.
- PostgreSQL, API e web: tutti **healthy**. Servizio migrazione **Exited (0)**, esito previsto per il processo one-shot. Volume PostgreSQL persistente creato.
- `docker compose exec -T api /api -healthcheck`: PASS, exit 0.
- `http://localhost:3000`: **200**; `/api/v1/auth/me` senza cookie: **401**.
- `PLAYWRIGHT_BASE_URL=http://localhost:3000 E2E_AUTH=1 npm run test:smoke`: **9/9 PASS, nessuno skip, 15,4 secondi**. Desktop, tablet e mobile; registrazione, cookie, refresh, logout, errore credenziali e nuovo login verificati contro i container reali.

Docker Desktop inizialmente richiedeva più del timeout del comando per partire, poi il motore è risultato pronto. Il primo pull non trovava `docker-credential-desktop` nel PATH del processo aperto prima dell’installazione; aggiunto il percorso dei tool Docker alla sola sessione e ripetuto con successo. Nessuna modifica globale al PATH. I servizi rimangono attivi per l’uso locale; arresto con `docker compose stop`, successivo avvio con `docker compose up -d --wait` dalla cartella con `compose.yaml`. Non sovrascrivere `.env` esistente.

## Non verificato / limiti noti

- **GitHub Actions non eseguita da remoto**, nessun push o pubblicazione. Race detector configurato in CI Linux; non eseguito nel run Windows locale.
- Rate limit 30 mutazioni/minuto per peer TCP, condiviso dietro Next. Non si fida di forwarded IP e non è distribuito: da rivedere prima della beta. Concorrenza Argon2 limitata a 4 worker.
- Nessuna verifica email, recupero password, profilo modificabile, follow, post reali, moderazione o GDPR operativo: rispettata la roadmap. Nessuna promessa di uso pubblico.
- Sessioni scadute/revocate conservate; pulizia e retention da definire prima della beta. Runner migrazioni minimo per versione 1; estenderlo quando si introduce la seconda migrazione.
- CSS nativo con token anziché introdurre Tailwind nel primo incremento: scelta di semplicità documentata in architettura. Nessun asset proprietario o screenshot storico copiato.

## Prossimo passo operativo

Aprire `http://localhost:3000` e creare un account locale. Il percorso Docker è ora verificato. Successivamente aprire **Sprint 2** (identità e grafo sociale) su richiesta; nessuna implementazione anticipata. Prima di pubblicare il repository, eseguire anche la pipeline GitHub Actions nel suo ambiente; nessun push o deploy incluso.
