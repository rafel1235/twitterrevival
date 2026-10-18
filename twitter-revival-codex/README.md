# Radura · Sprint 2

Social indipendente, nome interno **Twitter Revival**. Brand, testi, icone SVG e illustrazioni CSS originali. Nessuna affiliazione con Twitter/X, nessuno scraping o asset proprietario. Account, profili, follow e blocchi sono reali. I tre post della home sono etichettati **Demo**; composer e interazioni sui post restano disabilitati fino allo Sprint 3.

## Usare lo Sprint 2

- Sito: **http://localhost:3000**. Gli account creati nello Sprint 1 restano validi.
- **Impostazioni**: modifica nome, bio, località, sito e colore dell’avatar a iniziali; verifica email e accedi alla gestione dei blocchi.
- **Profilo**: contatori reali e liste follower/seguiti. **Persone** apre un profilo tramite handle esatto; la ricerca per parole chiave resta allo Sprint 4.
- **Segui / Non seguire più** e **Blocca / Sblocca** persistono nel database. Un blocco rimuove i follow reciproci e nasconde entrambi i profili. Sbloccare non ripristina i follow.
- **http://localhost:8025**: casella Mailpit locale. Qui arrivano i link di verifica e recupero password, senza invio di email su Internet. Le email automatiche sono reali messaggi SMTP catturati nell’ambiente di sviluppo.

Per provare due utenti, usare due browser/sessioni separate e aprire il profilo dell’altro account. Non esistono account amministratore o password universali predefiniti.

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

`001_auth.up.sql` crea `users`, `profiles`, `sessions`: CITEXT case-insensitive, UUID, foreign key con cascade, check di formato/dimensione/status/scadenza, indici e timestamp UTC (`TIMESTAMPTZ`). Utente, profilo e prima sessione vengono creati in una singola transazione. `002_identity_social.up.sql` aggiunge avatar_style, follows, blocks e auth_tokens, preservando gli account e le sessioni esistenti.

Il runner incorpora i file SQL nel binario, registra le versioni in `schema_migrations` e acquisisce un advisory lock transazionale. `up` applica tutte le versioni pendenti; `down` annulla **solo l’ultima versione**. Versioni sconosciute o non contigue sono rifiutate. Il rollback della 002 elimina connessioni e token email; una seconda esecuzione annulla la 001 e cancella anche l’autenticazione. Eseguire rollback solo con API ferma e backup adeguato:

```sh
cd services/api
go run ./cmd/server -migrate=down
go run ./cmd/server -migrate=up
```

`citext` è mantenuta al rollback perché potrebbe essere condivisa. Query parametrizzate in `db/queries/{auth,users,account_tokens}.sql`, codice tipizzato generato con **sqlc 1.31.1** in `internal/db` e usato dai repository. Per rigenerare, dalla directory `services/api`: `sqlc generate`. L’API richiede schema versione 2 prima di avviarsi. I test di upgrade/rollback usano schemi isolati; il database del sito non viene azzerato.

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

Le risposte auth includono inoltre `email_verified` (booleano) e `avatar_style` (colore placeholder). Questi dati, insieme all’email, appartengono alla risposta privata dell’account autenticato.

## API aggiunte nello Sprint 2

| Metodo e percorso                        | Input / risultato                                                                                         |
| ---------------------------------------- | --------------------------------------------------------------------------------------------------------- |
| `GET /api/v1/users/{handle}`             | Profilo visibile al chiamante; sessione obbligatoria; 404 per account assente, inattivo o bloccato.       |
| `PATCH /api/v1/users/me`                 | Campi opzionali: display_name, bio, location_text, website_url, avatar_style; 200 con profilo persistito. |
| `POST /api/v1/users/{id}/follow`         | `{}` → 204, idempotente; 403 se esiste un blocco.                                                         |
| `DELETE /api/v1/users/{id}/follow`       | `{}` → 204, idempotente.                                                                                  |
| `POST /api/v1/users/{id}/block`          | `{}` → 204; elimina entrambi i follow nella stessa transazione.                                           |
| `DELETE /api/v1/users/{id}/block`        | `{}` → 204; rimuove solo il blocco del chiamante.                                                         |
| `GET /api/v1/users/{id}/followers`       | Elenco paginato visibile al chiamante.                                                                    |
| `GET /api/v1/users/{id}/following`       | Elenco paginato visibile al chiamante.                                                                    |
| `GET /api/v1/users/me/blocks`            | Elenco privato degli account bloccati dal chiamante.                                                      |
| `POST /api/v1/auth/verification/request` | `{}`, sessione obbligatoria → 202; accoda una verifica se necessaria.                                     |
| `POST /api/v1/auth/verification/confirm` | `{token}` → 200; token scaduto, errato o consumato → 400.                                                 |
| `POST /api/v1/auth/password/forgot`      | `{email}` → 202 con risposta identica per account noti e ignoti.                                          |
| `POST /api/v1/auth/password/reset`       | `{token,password}` → 200; aggiorna Argon2id, revoca tutte le sessioni e scade il cookie corrente.         |

Tutte le mutazioni, comprese PATCH e DELETE, richiedono gli stessi controlli Origin/CSRF e JSON dello Sprint 1. Attore sempre derivato dalla sessione: nessun user_id nel body è accettato. Relazioni con sé stessi e UUID errati → 422; accesso anonimo a profili/liste → 401.

Il profilo restituisce `id`, `handle`, `display_name`, `bio`, `avatar_style`, `location_text`, `website_url`, `created_at`, `is_self`, `following`, `followed_by`, `followers_count`, `following_count`. Nessuna email o credenziale. Nome max 50 code point Unicode, bio 160, località 100; sito http/https senza credenziali, max 2048 byte. Avatar: sage, peach, lavender o sky. Il PATCH conserva i campi omessi o null; handle ed email non sono modificabili nello Sprint 2.

Le liste accettano `limit` (1–50, default 20) e `cursor` restituito dalla pagina precedente; risposta `{"data":{"users":[...],"next_cursor":"..."}}`, senza next_cursor alla fine. Ordine `created_at DESC, id DESC`; contatori e liste escludono account inattivi e relazioni bloccate per il visitatore. Per dettagli e scelte: [architettura Sprint 2](docs/architecture/sprint-2.md).

## Email e recupero password

Compose include Mailpit **1.31.6**, con interfaccia HTTP su loopback 8025 e SMTP su loopback 1025. Dall’API in container il server è `mailpit:1025`; con Go nativo il default è `127.0.0.1:1025`. `SMTP_FROM`, `SMTP_ADDR`, `SMTP_STARTTLS`, `SMTP_USERNAME`, `SMTP_PASSWORD` sono configurabili per l’avvio nativo. Compose è un ambiente locale con cattura email, senza relay Internet.

La registrazione accoda un link di verifica. I token hanno 32 byte casuali, hash SHA-256 nel DB, purpose specifico e uso singolo: 24 ore per verifica, 1 ora per reset. Massimo un nuovo token al minuto per account e tipo. Un nuovo invio invalida i precedenti. La conferma richiede un POST esplicito: aprire il link non lo consuma. Token esclusi dalle risposte API, dai log applicativi e dallo storage client; Referrer-Policy no-referrer, URL ripulito dopo il successo.

Il recupero accoda il lavoro prima del lookup dell’email, così account noti e ignoti ricevono lo stesso messaggio e percorso HTTP. Coda interna limitata a 32 richieste, timeout SMTP e cooldown nel DB. Il reset revoca tutte le sessioni e impedisce a un login concorrente con la vecchia password di crearne una nuova. Non effettua login automatico dopo il cambio.

**Limite noto:** la coda email non è durevole. Un riavvio può perdere richieste pendenti; richiedere un nuovo link dopo il cooldown. Errori di consegna vengono registrati senza dati sensibili. Per la beta servono consegna/retry durevoli e SMTP appropriato. SMTP autenticato richiede STARTTLS; con origin HTTPS STARTTLS è obbligatorio e i certificati vengono verificati. La verifica email è disponibile ma non ancora obbligatoria per login e connessioni nell’anteprima locale.

## Sicurezza

- Argon2id v19: memoria 64 MiB, 3 iterazioni, 2 lane, sale CSPRNG 16 byte, chiave 32 byte. PHC nel DB; confronto constant-time. Verifica dummy per email ignote/account sospesi. Massimo 4 operazioni Argon2 simultanee.
- Password: almeno 12 code point Unicode, massimo 128 byte UTF-8, mai normalizzate o troncate. Handle normalizzati lowercase: 3–20 caratteri ASCII `[A-Za-z0-9_]`. Nome visualizzato: 1–50 code point. Email validate e normalizzate lowercase.
- Token CSPRNG 32 byte, Base64URL; **solo SHA-256 del token nel DB**. Cookie HttpOnly, Path=/, SameSite=Lax, durata 168 ore configurabile (1–720). In HTTPS: Secure e nome `__Host-radura_session`, senza Domain. HTTP non-loopback e combinazioni cookie/origin insicure vengono rifiutati alla partenza.
- SameSite, origin esatto, Fetch Metadata e header custom anti-CSRF; nessun CORS aperto. Errori uniformi; `Cache-Control: no-store`; limiti body, header e timeout; nessuna credenziale nei log applicativi.
- Rate limit auth 30 mutazioni/minuto per **peer TCP**, memoria limitata a 4096 bucket. Non si fida di `X-Forwarded-For`. Dietro la rewrite Next i browser condividono la quota del proxy: limite conservativo da riprogettare con proxy fidato e quota per account prima della beta. Contatore locale al processo, azzerato al riavvio.

Le mutazioni social hanno quota separata di 60/minuto per peer; verifica e recupero 10/minuto. Valgono lo stesso limite di body, origin esatto e header anti-CSRF. Follow e blocchi sono serializzati nel DB mediante advisory lock della coppia; lo sblocco non ripristina relazioni. La sessione del frontend viene invalidata su 401 e le risposte precedenti a logout/reset non possono ripristinare un utente in cache.

Moderazione, cancellazione/esportazione dati e motore post restano negli sprint dedicati. Non è un servizio pronto per il pubblico. Nessun deploy incluso.

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
E2E_AUTH=1 E2E_MAILPIT=1 npm run test:smoke
```

PowerShell: `$env:E2E_AUTH='1'; $env:E2E_MAILPIT='1'; npm run test:smoke`. Per verificare lo stack Compose già avviato impostare anche `$env:PLAYWRIGHT_BASE_URL='http://localhost:3000'` (disabilita il webServer automatico). `E2E_AUTH` abilita i cicli reali auth/social; `E2E_MAILPIT` abilita il ciclo verifica/reset tramite SMTP reale. Senza le rispettive variabili i test dipendenti sono esplicitamente saltati. `MAILPIT_URL` può cambiare l’URL della casella per i test (default http://localhost:8025).

Screenshot a 1440×900, 768×1024, 390×844 in `test-results/`; report con `npx playwright show-report`. La suite include persistenza del profilo e avatar, validazione Unicode, nomi di 50 caratteri e bio di 160 emoji senza overflow, follow/unfollow reciproci, blocco/sblocco, verifica email, password reset e revoca dei cookie precedenti. Nessun mock/intercettazione delle API nei test browser.

La CI GitHub Actions avvia PostgreSQL e Mailpit, esegue format/lint/tipi/build, Go vet/test con race detector e il ciclo Playwright completo. `.github/workflows/ci.yml` serve per pubblicare questa directory come repository autonomo; `../.github/workflows/radura-ci.yml` è il workflow nella root Git effettiva della cartella contenitore `twitter revival`, con working-directory e cache configurate per il progetto annidato. L’esecuzione remota richiede pubblicare il repository e non viene dichiarata verificata localmente.

## Struttura essenziale

```text
apps/web/src/app/                 pagine App Router e design token
apps/web/src/components/          layout, auth, timeline demo
apps/web/src/components/profile/  profilo, editor, connessioni, blocchi, persone
apps/web/tests/                   smoke e percorso auth reale
services/api/cmd/server/          API e comandi migrazione/healthcheck
services/api/internal/auth/       servizio e repository PostgreSQL
services/api/internal/users/      profili e grafo sociale
services/api/internal/mailer/     invio SMTP con timeout e STARTTLS
services/api/internal/httpapi/    handler, CSRF, rate limit, test
services/api/internal/db/         query Go generate da sqlc
services/api/db/queries/          SQL sorgente
services/api/migrations/          schema, rollback, runner embedded
docs/                            architettura, provenienza UI, stato verifiche
.github/workflows/ci.yml          pipeline
compose.yaml                     db/mailpit → migrate → api → web
```

Compose è stato avviato e verificato localmente con database, API, web e Mailpit sani; migrazione completata con exit 0. Esiti e file dello Sprint 2 in [docs/implementation-status.md](docs/implementation-status.md). Il prossimo incremento è Sprint 3 per post e timeline reale, solo su richiesta.
