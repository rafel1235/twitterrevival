# Radura · Sprint 1

Social indipendente, nome interno **Twitter Revival**. Brand, testi, icone SVG e illustrazioni CSS originali. Nessuna affiliazione con Twitter/X, nessuno scraping o asset proprietario. Account e sessioni sono reali; i tre post inventati sono etichettati **Demo**. Composer e interazioni sono disabilitati fino allo Sprint 3.

## Avvio con Docker Compose

Prerequisiti: Docker Engine/Desktop con Compose v2 o successivo. Dalla directory che contiene questo README:

```sh
cp .env.example .env
# Cambiare POSTGRES_PASSWORD in .env con una password locale, URL-safe.
docker compose up --build
```

Aprire **http://localhost:3000** (usare localhost, non 127.0.0.1: l’origin deve coincidere). Compose aspetta PostgreSQL sano, esegue le migrazioni, avvia API e web. Le API non sono pubblicate sull’host; il browser le raggiunge tramite Next. PostgreSQL è esposto solo sul loopback per lo sviluppo. Il volume persiste dopo `docker compose down`.

PowerShell: usare `Copy-Item .env.example .env` al posto di `cp`. Se si cambia `WEB_PORT`, aggiornare anche `APP_ORIGIN`. Password con caratteri riservati agli URL richiedono percent-encoding nel DSN; per il setup locale usare una password esadecimale.

**Se `.env` esiste già, conservarlo:** non ricopiarlo dall’esempio e non cambiare la password di un database già inizializzato. Il file contiene solo configurazione locale ed è escluso da Git.

Su Windows aprire Docker Desktop e attendere che il motore sia pronto prima dei comandi Compose ([avvio ufficiale](https://docs.docker.com/desktop/setup/install/windows-install/#start-docker-desktop)). Dopo una nuova installazione, riavviare VS Code/il terminale se `docker` o `docker-credential-desktop` non vengono trovati nel PATH.

Per gli avvii successivi, dalla directory con `compose.yaml`:

```sh
docker compose up -d --wait
# Quando si vuole fermare il progetto, conservando i dati:
docker compose stop
```

```sh
docker compose ps
docker compose logs api migrate web
docker compose exec api /api -healthcheck
docker compose down
```

`docker compose down -v` elimina **tutti i dati locali del progetto**: usarlo solo se si desidera ripartire da zero.

## Sviluppo senza Docker

Prerequisiti: Node 24, npm 11+, Go 1.27+, PostgreSQL 18 con estensione `citext` disponibile. Le versioni effettivamente verificate sono registrate in [docs/implementation-status.md](docs/implementation-status.md); `package-lock.json` e `services/api/go.sum` fissano le dipendenze.

Creare un database e un utente PostgreSQL dedicati. Esportare queste variabili nella shell dell’API (il processo Go **non** carica automaticamente `.env`):

```sh
export DATABASE_URL='postgres://radura:LOCAL_PASSWORD@localhost:5432/radura?sslmode=disable'
export APP_ORIGIN='http://localhost:3000'
export COOKIE_SECURE=false
cd services/api
go run ./cmd/server -migrate=up
go run ./cmd/server
```

In un secondo terminale dalla root:

```sh
npm ci
npm run dev
```

Equivalente PowerShell per le variabili: `$env:DATABASE_URL='postgres://...'`, `$env:APP_ORIGIN='http://localhost:3000'`, `$env:COOKIE_SECURE='false'`. I comandi Go e npm restano uguali.

`API_INTERNAL_URL` (default `http://127.0.0.1:8080`) configura la rewrite Next `/api/v1/:path*`. Deve essere presente nell’ambiente di Next all’avvio dev o **al build**; in Compose è impostato come build arg a `http://api:8080`. Anche `NEXT_PUBLIC_PRODUCT_NAME` è configurabile al build. Dopo un cambio di questi valori ricompilare il frontend. Il client usa esclusivamente URL relativi; non occorrono CORS o token in localStorage.

Build locale di produzione:

```sh
npm run build
npm run start
```

## Migrazioni e query

`001_auth.up.sql` crea `users`, `profiles`, `sessions`: CITEXT case-insensitive, UUID, foreign key con cascade, check di formato/dimensione/status/scadenza, indici e timestamp UTC (`TIMESTAMPTZ`). Utente, profilo e prima sessione vengono creati in una singola transazione.

Il runner incorpora i file SQL nel binario, registra la versione in `schema_migrations` e acquisisce un advisory lock transazionale. `up` e `down` ripetuti sono idempotenti. Per rollback **distruttivo dei dati di autenticazione**, con API ferma e backup adeguato:

```sh
cd services/api
go run ./cmd/server -migrate=down
go run ./cmd/server -migrate=up
```

`citext` è mantenuta al rollback perché potrebbe essere condivisa. Query parametrizzate in `db/queries/auth.sql`, codice tipizzato generato con **sqlc 1.31.1** in `internal/db` e usato dal repository. Per rigenerare, dalla directory `services/api`: `sqlc generate`.

## Contratto HTTP

| Metodo e percorso            | Risultato                                                                                |
| ---------------------------- | ---------------------------------------------------------------------------------------- |
| `GET /healthz`               | `200 {"data":{"status":"ok"}}`; `503` se DB/schema non pronti. Accesso diretto alla API. |
| `POST /api/v1/auth/register` | `201`, utente e cookie sessione; conflitto email/handle `409`.                           |
| `POST /api/v1/auth/login`    | `200`, utente e nuova sessione; credenziali errate `401`.                                |
| `POST /api/v1/auth/logout`   | `204`, revoca sessione corrente e scade cookie; idempotente.                             |
| `GET /api/v1/auth/me`        | `200`, utente corrente; sessione assente, scaduta, revocata o account sospeso `401`.     |

Successi auth: `{"data":{"user":{"id":"uuid","handle":"ada","email":"ada@example.test","display_name":"Ada","created_at":"ISO-8601 UTC"}}}`. Errori: `{"error":{"code":"validation_failed","message":"Controlla i dati inseriti.","fields":{"handle":"..."}}}`. Nessun hash, token o dettaglio interno nel JSON. `fields` è opzionale. `logout` non ha body di risposta.

Registrazione: `handle`, `email`, `display_name`, `password`. Login: `email`, `password`. Logout: `{}`. Tutte le mutazioni richiedono `Content-Type: application/json`, `X-CSRF-Protection: 1` e `Origin` uguale ad `APP_ORIGIN` (fallback `Referer` solo se Origin manca). Origin assente/non valido o metadata cross-site: `403`; tipo errato `415`; JSON invalido/campi ignoti `400`; body oltre 8 KiB `413`; input non valido `422`; quota superata `429` con `Retry-After`; saturazione worker `503`.

Esempio da shell, API locale:

```sh
curl -i -c cookies.txt http://localhost:8080/api/v1/auth/register \
  -H 'Origin: http://localhost:3000' -H 'X-CSRF-Protection: 1' \
  -H 'Content-Type: application/json' \
  --data '{"handle":"ada_riva","email":"ada@example.test","display_name":"Ada","password":"a unique local password"}'
curl -b cookies.txt http://localhost:8080/api/v1/auth/me
curl -i -b cookies.txt -c cookies.txt http://localhost:8080/api/v1/auth/logout \
  -H 'Origin: http://localhost:3000' -H 'X-CSRF-Protection: 1' \
  -H 'Content-Type: application/json' --data '{}'
```

## Sicurezza del primo incremento

- Argon2id v19: memoria 64 MiB, 3 iterazioni, 2 lane, sale CSPRNG 16 byte, chiave 32 byte. PHC nel DB; confronto constant-time. Verifica dummy per email ignote/account sospesi. Massimo 4 operazioni Argon2 simultanee.
- Password: almeno 12 code point Unicode, massimo 128 byte UTF-8, mai normalizzate o troncate. Handle normalizzati lowercase: 3–20 caratteri ASCII `[A-Za-z0-9_]`. Nome visualizzato: 1–50 code point. Email validate e normalizzate lowercase.
- Token CSPRNG 32 byte, Base64URL; **solo SHA-256 del token nel DB**. Cookie HttpOnly, Path=/, SameSite=Lax, durata 168 ore configurabile (1–720). In HTTPS: Secure e nome `__Host-radura_session`, senza Domain. HTTP non-loopback e combinazioni cookie/origin insicure vengono rifiutati alla partenza.
- SameSite, origin esatto, Fetch Metadata e header custom anti-CSRF; nessun CORS aperto. Errori uniformi; `Cache-Control: no-store`; limiti body, header e timeout; nessuna credenziale nei log applicativi.
- Rate limit auth 30 mutazioni/minuto per **peer TCP**, memoria limitata a 4096 bucket. Non si fida di `X-Forwarded-For`. Dietro la rewrite Next i browser condividono la quota del proxy: limite conservativo da riprogettare con proxy fidato e quota per account prima della beta. Contatore locale al processo, azzerato al riavvio.

Niente verifica email, recupero password, moderazione o cancellazione/esportazione dati in questo sprint. Non è un servizio pronto per il pubblico. Nessun deploy incluso.

## Verifiche riproducibili

```sh
npm run format:check
npm run lint
npm run typecheck
npm run build
cd services/api
gofmt -l cmd internal migrations
go vet ./...
go test -count=1 -v ./...
go build ./cmd/server
```

Con PostgreSQL dedicato, esportare `TEST_DATABASE_URL` prima di `go test`: i test reali creano uno schema isolato e lo eliminano alla fine; **non** usare un database di produzione. Richiedono permessi `CREATE SCHEMA` e `CREATE EXTENSION citext`. Senza variabile il test di integrazione è esplicitamente saltato. Il ciclo include hash password/token, case-insensitive conflict, sessione dopo nuova istanza, scadenza, sospensione, logout, constraint, atomicità e rollback/up delle migrazioni.

Playwright avvia `npm run start`, quindi serve prima `npm run build`. Con API e database attivi:

```sh
npx playwright install chromium
E2E_AUTH=1 npm run test:smoke
```

PowerShell: `$env:E2E_AUTH='1'; npm run test:smoke`. Per verificare lo stack Compose già avviato: impostare anche `PLAYWRIGHT_BASE_URL=http://localhost:3000` (disabilita il webServer automatico). Senza `E2E_AUTH=1` i test UI girano e il ciclo auth viene esplicitamente saltato. Gli smoke richiedono comunque l’API per il controllo iniziale della sessione. Screenshot a 1440×900, 768×1024, 390×844 in `test-results/`; report con `npx playwright show-report`.

La CI GitHub Actions avvia PostgreSQL reale, esegue format/lint/tipi/build, Go vet/test con race detector e il ciclo Playwright completo. La configurazione CI è inclusa; l’esecuzione remota richiede pubblicare il repository e non viene dichiarata verificata localmente.

## Struttura essenziale

```text
apps/web/src/app/                 pagine App Router e design token
apps/web/src/components/          layout, auth, timeline demo
apps/web/tests/                   smoke e percorso auth reale
services/api/cmd/server/          API e comandi migrazione/healthcheck
services/api/internal/auth/       servizio e repository PostgreSQL
services/api/internal/httpapi/    handler, CSRF, rate limit, test
services/api/internal/db/         query Go generate da sqlc
services/api/db/queries/          SQL sorgente
services/api/migrations/          schema, rollback, runner embedded
docs/                            architettura, provenienza UI, stato verifiche
.github/workflows/ci.yml          pipeline
compose.yaml                     db → migrate → api → web
```

Compose è stato avviato e verificato localmente con tutti i servizi sani e 9/9 test Playwright superati contro i container reali; dettagli in `docs/implementation-status.md`. Il prossimo incremento è Sprint 2 per profili e grafo sociale, su richiesta. Il motore post appartiene allo Sprint 3.
