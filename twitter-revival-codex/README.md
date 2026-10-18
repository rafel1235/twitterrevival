# Radura · Sprint 5

Social indipendente, nome interno **Twitter Revival**. Brand, testi, icone SVG e illustrazioni CSS originali. Nessuna affiliazione con Twitter/X, nessuno scraping o asset proprietario. Account, profili, follow, blocchi, post e interazioni sono reali e persistenti. La home autenticata mostra il feed cronologico dei seguiti e i propri post. Senza accesso compare solo un'anteprima esplicitamente **Demo**.

## Usare lo Sprint 5

- **Segnala post/profilo**: apri il controllo sotto un contenuto altrui, scegli motivo e dettagli; l'esito compare in **Impostazioni → Segnalazioni**.
- **Moderazione** (`/moderation`): coda, oscuramento/ripristino post, sospensione/riattivazione account e audit. Ruolo assegnato solo con CLI locale esplicita, vedi sotto.
- **Impostazioni → I tuoi dati**: ZIP reale con JSON e immagini, oppure cancellazione definitiva confermata da password e parola ELIMINA. Nessun dato simulato.
- **Backup e restore**: `./scripts/backup-verify.ps1` prova una copia isolata di database e storage; nessun restore sui dati attivi. I tempi tecnici di conservazione sono spiegati in `/privacy` e nella documentazione.

- Sito: **http://localhost:3000**. Gli account creati nello Sprint 1 restano validi.
- **Home dopo l'accesso**: pubblica fino a 280 code point Unicode. “Aggiorna” legge i nuovi post; “Carica altri post” prosegue con il cursore. Il feed include post propri e dei seguiti, risposte, citazioni e repost, in ordine cronologico.
- **Post**: apri il testo/data per la conversazione; rispondi, metti/rimuovi mi piace, ripubblica/annulla repost o cita con un tuo commento. Puoi modificare solo il testo dei tuoi post ed eliminarli con conferma. I post restano dopo refresh/riavvio.
- **Aggiungi immagine**: seleziona JPEG/PNG, descrivila e premi “Carica immagine”. Fino a quattro allegati con testo obbligatorio; recupero e cancellazione delle immagini in bozza anche dopo refresh.
- **Esplora**: ricerca parole nei post, handle o nome nelle persone; risultati paginati con filtri per blocchi e account attivi.
- **Notifiche**: follow, like, repost, risposte e citazioni persistenti. Badge/pagina aggiornati ogni 30 secondi, lettura singola o tutte. Nessuna notifica verso sé stessi.
- **Impostazioni**: modifica nome, bio, località, sito e colore dell’avatar a iniziali; verifica email e accedi alla gestione dei blocchi.
- **Profilo**: contatori e liste follower/seguiti reali. **Persone** apre ancora un profilo tramite handle esatto; **Esplora** offre la ricerca per parole chiave.
- **Segui / Non seguire più** e **Blocca / Sblocca** persistono nel database. Un blocco rimuove i follow reciproci e nasconde entrambi i profili. Sbloccare non ripristina i follow.
- **http://localhost:8025**: casella Mailpit locale. Qui arrivano i link di verifica e recupero password, senza invio di email su Internet. Le email automatiche sono reali messaggi SMTP catturati nell’ambiente di sviluppo.

Per provare due utenti, usare due browser/sessioni separate e aprire il profilo dell’altro account. Non esistono account amministratore o password universali predefiniti.

## Avvio con Docker Compose

Prerequisiti: Docker Engine/Desktop con Compose v2 o successivo. Dalla directory che contiene questo README:

```sh
cp .env.example .env
# Cambiare POSTGRES_PASSWORD e S3_SECRET_KEY con segreti locali distinti.
docker compose up --build
```

Aprire **http://localhost:3000** (usare localhost, non 127.0.0.1: l’origin deve coincidere). Compose aspetta PostgreSQL sano, esegue le migrazioni, avvia API e web. Le API non sono pubblicate sull’host; il browser le raggiunge tramite Next. PostgreSQL è esposto solo sul loopback per lo sviluppo. Il volume persiste dopo `docker compose down`.

PowerShell: usare `Copy-Item .env.example .env` al posto di `cp`. Se si cambia `WEB_PORT`, aggiornare anche `APP_ORIGIN`. Password con caratteri riservati agli URL richiedono percent-encoding nel DSN; per il setup locale usare una password esadecimale.

**Se `.env` esiste già, conservarlo:** non ricopiarlo dall’esempio e non cambiare la password di un database già inizializzato. Il file contiene solo configurazione locale ed è escluso da Git.

Per aggiornare dallo Sprint 3 aggiungere solo le variabili S3 mancanti indicate in `.env.example`. `S3_SECRET_KEY` deve essere casuale, di almeno 32 byte; in questa installazione è già configurato. Esempio PowerShell 7 per aggiungerlo solo se assente, senza stamparlo:

```powershell
if ((Get-Content -Raw .env) -notmatch '(?m)^S3_SECRET_KEY=') {
  $storageSecret = [Convert]::ToHexString([Security.Cryptography.RandomNumberGenerator]::GetBytes(32))
  Add-Content .env "`nS3_SECRET_KEY=$storageSecret"
}
docker compose up --build -d --wait
```

Compose aggiunge SeaweedFS **4.48** con volume persistente `media_data`, S3 solo su `127.0.0.1:8333`; console/admin non pubblicati. Bucket privato inizializzato dalla API. Immagini nel browser soltanto attraverso l'API autenticata; nessun account AWS/cloud necessario. Conservare anche il volume media: il solo dump PostgreSQL non contiene i byte delle immagini.

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

Prerequisiti: Node 24, npm 11+, Go 1.27+, PostgreSQL 18 con `citext` e un endpoint S3 compatibile privato. Per tenere solo lo storage in Compose: `docker compose up -d --wait storage`. Versioni verificate nel registro; lockfile npm e `services/api/go.sum` fissano le dipendenze, incluso SDK AWS Go v2 (S3 1.114.3, core 1.47.3, credentials 1.20.9).

Creare un database e un utente PostgreSQL dedicati. Esportare queste variabili nella shell dell’API (il processo Go **non** carica automaticamente `.env`):

```sh
export DATABASE_URL='postgres://radura:LOCAL_PASSWORD@localhost:5432/radura?sslmode=disable'
export APP_ORIGIN='http://localhost:3000'
export COOKIE_SECURE=false
export S3_ENDPOINT='http://127.0.0.1:8333'
export S3_REGION='us-east-1'
export S3_BUCKET='radura-media'
export S3_ACCESS_KEY='radura-local'
export S3_SECRET_KEY='SAME_RANDOM_LOCAL_SECRET_AS_STORAGE'
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

`003_posts.up.sql` aggiunge posts, likes, reposts, view di visibilità e indici per feed/risposte. Conserva tutti gli account, le sessioni e le connessioni esistenti.

`004_discovery_media.up.sql` aggiunge media/post_media, notifiche con trigger/view, campo generato FTS e indice GIN. Conserva post e interazioni esistenti; nessun backfill delle notifiche storiche.

Il runner incorpora SQL, registra `schema_migrations` e acquisisce un advisory lock transazionale. `up` applica le versioni pendenti; `down` annulla **solo l'ultima**. Versioni sconosciute/non contigue rifiutate. Il primo rollback annulla la 005: elimina report/audit/quote/ruoli e rimuove l'oscuramento, preservando account/post/media. Richiede il binario dello sprint precedente e API ferme; non recupera dati già cancellati dalla manutenzione. I successivi annullano 004, 003, 002 e 001 con perdita dei rispettivi dati. Salvare un backup prima:

```sh
cd services/api
go run ./cmd/server -migrate=down
go run ./cmd/server -migrate=up
```

`citext` resta al rollback perché potrebbe essere condivisa. Query parametrizzate in `db/queries/`, codice tipizzato generato con **sqlc 1.31.1** in `internal/db`. Per rigenerare, da `services/api`: `sqlc generate`. API con schema versione **5**; healthcheck su DB/schema e S3. Test rollback su schemi isolati, senza azzerare il sito.

## Contratto HTTP

| Metodo e percorso            | Risultato                                                                                |
| ---------------------------- | ---------------------------------------------------------------------------------------- |
| `GET /healthz`               | `200 {"data":{"status":"ok"}}`; `503` se DB/schema non pronti. Accesso diretto alla API. |
| `POST /api/v1/auth/register` | `201`, utente e cookie sessione; conflitto email/handle `409`.                           |
| `POST /api/v1/auth/login`    | `200`, utente e nuova sessione; credenziali errate `401`.                                |
| `POST /api/v1/auth/logout`   | `204`, revoca sessione corrente e scade cookie; idempotente.                             |
| `GET /api/v1/auth/me`        | `200`, utente corrente; sessione assente, scaduta, revocata o account sospeso `401`.     |

Successi auth: `{"data":{"user":{"id":"uuid","handle":"ada","email":"ada@example.test","display_name":"Ada","created_at":"ISO-8601 UTC"}}}`. Errori: `{"error":{"code":"validation_failed","message":"Controlla i dati inseriti.","fields":{"handle":"..."}}}`. Nessun hash, token o dettaglio interno nel JSON. `fields` è opzionale. `logout` non ha body di risposta.

Registrazione: `handle`, `email`, `display_name`, `password`. Login: `email`, `password`. Logout: `{}`. Mutazioni con `Content-Type: application/json` (upload: multipart), `X-CSRF-Protection: 1` e `Origin` uguale ad `APP_ORIGIN` (fallback Referer solo se Origin manca). Origin non valido/metadata cross-site: 403; tipo errato 415; JSON/campi ignoti 400; body JSON oltre 8 KiB 413; validazione 422; quota 429 con Retry-After; saturazione 503. Limite multipart distinto descritto sotto.

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

## API aggiunte nello Sprint 3

Tutti gli endpoint seguenti richiedono una sessione valida, anche in lettura. Le mutazioni richiedono gli header Origin/CSRF e JSON descritti sopra.

| Metodo e percorso                   | Input / risultato                                                                                                         |
| ----------------------------------- | ------------------------------------------------------------------------------------------------------------------------- |
| `POST /api/v1/posts`                | `{body, reply_to_id?, quote_of_id?}` → 201, `Location` e `data.post`. Testo obbligatorio; al massimo uno dei riferimenti. |
| `GET /api/v1/posts/{id}`            | 200 con `data.post`; 404 per assente/cancellato/bloccato/inattivo.                                                        |
| `PATCH /api/v1/posts/{id}`          | `{body}` → 200; solo proprietario e solo testo, 403 se appartiene ad altri.                                               |
| `DELETE /api/v1/posts/{id}`         | `{}` → 204; soft delete, ripetibile dal proprietario.                                                                     |
| `POST /api/v1/posts/{id}/likes`     | `{}` → 200 con card aggiornata; idempotente.                                                                              |
| `DELETE /api/v1/posts/{id}/likes`   | `{}` → 200 con card aggiornata; idempotente.                                                                              |
| `POST /api/v1/posts/{id}/reposts`   | `{}` → 200 con card aggiornata; idempotente.                                                                              |
| `DELETE /api/v1/posts/{id}/reposts` | `{}` → 200 con card aggiornata; idempotente.                                                                              |
| `GET /api/v1/posts/{id}/replies`    | Risposte dirette visibili, paginate; 404 se l'originale non è visibile.                                                   |
| `GET /api/v1/feed/home`             | Eventi propri e dei seguiti, inclusi repost.                                                                              |
| `GET /api/v1/users/{id}/posts`      | Post e repost del profilo visibile.                                                                                       |

Le liste accettano `limit` (1–50, default 20) e `cursor`; risposta `{"data":{"items":[{"id":"event-uuid","created_at":"UTC","post":{},"reposted_by":{}}],"next_cursor":"..."}}`. `reposted_by` compare solo per i repost, `next_cursor` solo quando serve. Ordine discendente per data e UUID dell'evento, senza ranking. Originale e repost sono eventi distinti: un post può apparire due volte se si seguono entrambi gli autori dell'evento. Le risposte usano lo stesso envelope, senza `reposted_by`.

La card restituisce `id`, `author` (id/handle/display_name/avatar_style), `body`, `created_at`, `updated_at`, `is_own`, `liked`, `reposted`, i quattro contatori `likes_count`, `reposts_count`, `replies_count`, `quotes_count` e gli eventuali `reply_to_id`/`quote_of_id`. `reply_to` e `quote_of` contengono una preview visibile di un solo livello, oppure null. Non includono email o credenziali. I contatori e gli incorporamenti rispettano blocchi e account inattivi.

Testo da 1 a 280 code point Unicode dopo trim, newline/tab ammessi; controlli vietati. Client e server contano nello stesso modo. Modificare il testo mantiene l'ordine originale; la UI indica “Modificato”. Riferimenti e autore sono immutabili. Le citazioni richiedono un commento proprio; per ripubblicare senza commento usare repost. Un originale cancellato, oscurato o bloccato è nascosto nelle citazioni/risposte altrui. Un blocco conserva le vecchie righe like/repost, ma le filtra; dopo sblocco possono riapparire. Il singolo DELETE mantiene il testo per 30 giorni, poi la manutenzione lo sostituisce; cancellazione account immediata del testo. Copie di backup con retention separata.

“Aggiorna” o refresh legge le azioni effettuate da un'altra sessione. Dopo una propria mutazione le liste ripartono dalla prima pagina; nessuna sincronizzazione realtime in questo sprint. Per transazioni, lock e politiche complete: [architettura Sprint 3](docs/architecture/sprint-3.md).

## API aggiunte nello Sprint 4

Tutte richiedono sessione. Mutazioni con origin/CSRF; upload multipart, altre mutazioni JSON `{}`.

| Metodo e percorso                        | Risultato                                                        |
| ---------------------------------------- | ---------------------------------------------------------------- |
| `POST /api/v1/media`                     | Multipart `file` + `alt_text` → 201 con `data.media`.            |
| `GET /api/v1/media/drafts`               | `data.media[]`: proprie bozze, `available_until` e `attachable`. |
| `GET /api/v1/media/{id}`                 | Metadati visibili in `data.media`.                               |
| `GET /api/v1/media/{id}/content`         | Byte immagine con cookie e visibilità; nessun URL S3 pubblico.   |
| `DELETE /api/v1/media/{id}`              | 204 per bozza propria; 409 se associata, 404 se altrui.          |
| `GET /api/v1/search?q=…&type=posts`      | Post visibili in `data.items`, cursore opzionale.                |
| `GET /api/v1/search?q=…&type=users`      | Persone visibili in `data.users`, cursore opzionale.             |
| `GET /api/v1/notifications`              | `data.items`, `next_cursor?`, `unread_count`.                    |
| `GET /api/v1/notifications/unread-count` | `data.unread_count`.                                             |
| `POST /api/v1/notifications/{id}/read`   | 204; solo destinatario, ripetibile, 404 se altrui/non visibile.  |
| `POST /api/v1/notifications/read`        | 204; marca tutte quelle visibili nello snapshot della query.     |

Upload: JPEG/PNG max 5 MiB, 4096 px per lato/8 megapixel, ricodifica e rimozione metadati, output max 8 MiB. Alt obbligatorio 1–200 code point; SVG/GIF e MIME incongruente → 415, dimensione byte → 413, dati/limiti immagine → 422. Massimo 12 bozze e 100 MiB attivi/account, due elaborazioni simultanee e 20 mutazioni/minuto per peer. Bozze associabili per 24 ore, poi ancora eliminabili manualmente. `POST /posts` accetta `media_ids` (max quattro ID propri, unici, caricati e non riutilizzati). Card/preview hanno `media[]` con `id,url,mime_type,bytes,width,height,alt_text`. Testo sempre obbligatorio; allegati immutabili dopo pubblicazione.

Ricerca: q di 2–100 code point, limite 20/max 50 e cursori esistenti. Post tramite FTS simple/websearch, senza stemming o rimozione accenti; persone per prefisso handle o parte del nome, senza email. Wildcard LIKE letterali; ordine cronologico, nessun ranking; 120 ricerche/minuto per peer.

Notifiche atomiche per nuove azioni follow/like/repost/reply/quote, senza notifiche verso sé stessi o retroattive. DTO: `id,type,actor,post_id?,created_at,read_at` nullable. Cursori e limite 20/max 50. Undo, cancellazione post o blocco rimuovono le notifiche collegate; sblocco senza ripristino. Filtri di visibilità a ogni lettura. Polling 30 secondi con scheda visibile; lista riletta dalla prima pagina. Quota mutazioni stato lettura 60/minuto per peer.

Purge S3 ora durevole e automatico: bozze 24h, singoli post eliminati 30 giorni, account eliminato coda immediata. Riconciliazione di oggetti senza metadati tramite CLI esplicita. Avatar restano placeholder; niente video, WebP o upload profilo. Dettagli aggiornati nell'[architettura Sprint 5](docs/architecture/sprint-5.md).

## API e comandi aggiunti nello Sprint 5

Sessione obbligatoria. Mutazioni con JSON e `Origin`/`X-CSRF-Protection: 1`. Campi sconosciuti rifiutati.

| Metodo e percorso                                | Input / risposta                                                                                                                                                                                       |
| ------------------------------------------------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `POST /api/v1/reports`                           | `{target_type: "post"\|"user", target_id, reason, details}` → 201 `data.id`; motivo `spam\|harassment\|illegal\|other`, dettagli 10–2000 code point. Report aperto duplicato restituisce lo stesso ID. |
| `GET /api/v1/reports`                            | Solo segnalazioni proprie: `data.items,next_cursor?`; filtro `status=open\|resolved\|dismissed`, limit/cursor.                                                                                         |
| `GET /api/v1/moderation/reports`                 | Coda ed evidenze; solo moderatori, stessi filtri/cursori.                                                                                                                                              |
| `POST /api/v1/moderation/reports/{id}/decisions` | `{action,reason}` → 204; motivo 10–2000 caratteri. Azioni `dismiss,hide_post,restore_post,suspend_user,restore_user`.                                                                                  |
| `GET /api/v1/moderation/audit`                   | Audit paginato `data.items,next_cursor?`; solo moderatori.                                                                                                                                             |
| `POST /api/v1/account/export`                    | `{password}` → ZIP `data.json` + immagini proprie, no-store; password errata 403.                                                                                                                      |
| `DELETE /api/v1/account`                         | `{password,confirmation:"ELIMINA"}` → 204; revoca sessioni, pulisce dati e accoda purge media.                                                                                                         |

I DTO auth includono `role: member|moderator`. Nessuna assegnazione dal client. Per abilitare **un handle esistente scelto dal gestore** (sostituire HANDLE, nessuna password universale):

```sh
docker compose exec api /api -role-user HANDLE -role moderator -reason "Assegnazione del ruolo approvata dal gestore locale."
# Per revocare: stesso comando con -role member e motivazione.
docker compose exec api /api -maintenance
docker compose exec api /api -reconcile-media
```

Ogni cambio ruolo è registrato nell'audit. Ricaricare la pagina dopo l'assegnazione; aprire http://localhost:3000/moderation. `-maintenance` segnala eliminazioni S3 ancora pendenti; retry automatico ogni minuto. `-reconcile-media` elimina solo orfani oltre 24h senza record DB.

Export automatico: massimo 100 MiB immagini, 32 MiB JSON, 10.000 elementi per categoria; superamento 413 senza troncamenti. File temporaneo eliminato dopo la risposta. Testo eliminato dall'account subito, tombstone UUID conservati per i riferimenti; report/audit limitati secondo policy provvisoria. Nessuna promessa di cancellazione istantanea dai backup.

```powershell
./scripts/backup-verify.ps1
```

Breve pausa delle scritture, dump PostgreSQL e snapshot SeaweedFS; restore in rete/volumi separati, confronto SHA-256 di tutte le righe e immagini, ricevuta e pulizia dei soli volumi temporanei. Cartella privata `.local/backups/sprint5`, esclusa da Git. Procedura e limiti: [backup/restore](docs/operations/backup-restore.md). [Review rischi](docs/security/sprint-5-review.md) e [bozza privacy/DSA](docs/legal/privacy-dsa-review.md) da rivedere prima del lancio; nessuna dichiarazione di conformità.

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

Restano le quote per peer e i controlli Origin/CSRF degli sprint precedenti. In più quote persistenti al minuto per account: 30 pubblicazioni, 20 operazioni media, 10 report, 3 export, 5 tentativi cancellazione, 120 altre scritture. Auth 30/minuto per peer anche dopo riavvio. Duplicati testuali sullo stesso riferimento rifiutati per 10 minuti (409); 429 con Retry-After per quota. Next condivide il peer: limite conservativo, X-Forwarded-For non attendibile. Lock account ordinati prima di coppia/post; cancellazioni e sospensioni seguono lo stesso ordine. Log inattesi solo per classe, senza errori driver o path grezzi. [Politiche complete](docs/architecture/sprint-5.md).

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

Playwright avvia `npm run start`, quindi serve prima `npm run build`. Gli helper auth rispettano eventuali 429 attendendo il `Retry-After` del server: i limiti rimangono attivi anche nei test. La suite può quindi richiedere oltre un minuto. Con API e database attivi:

```sh
npx playwright install chromium
E2E_AUTH=1 E2E_MAILPIT=1 npm run test:smoke
```

PowerShell: `$env:E2E_AUTH='1'; $env:E2E_MAILPIT='1'; npm run test:smoke`. Per verificare lo stack Compose già avviato impostare anche `$env:PLAYWRIGHT_BASE_URL='http://localhost:3000'` (disabilita il webServer automatico). `E2E_AUTH` abilita i cicli reali auth/social; `E2E_MAILPIT` abilita il ciclo verifica/reset tramite SMTP reale. Senza le rispettive variabili i test dipendenti sono esplicitamente saltati. `MAILPIT_URL` può cambiare l’URL della casella per i test (default http://localhost:8025).

Screenshot a 1440×900, 768×1024, 390×844 in `test-results/`; report con `npx playwright show-report`. La suite mantiene auth, profili/avatar, follow/blocchi e SMTP dello Sprint 2. Aggiunge pubblicazione con 280 emoji (281 rifiutate), modifica e cancellazione proprie, testo HTML innocuo, like/repost, risposta/citazione, oscuramento dopo blocco e persistenza dopo refresh. La paginazione oltre 20 eventi viene verificata anche nella viewport desktop. Nessun mock/intercettazione delle API nei test browser. I test Go aggiungono vincoli SQL, cursori a timestamp uguali, inserimenti fra pagine, account sospesi e scritture concorrenti con blocco/cancellazione.

La CI GitHub Actions avvia PostgreSQL e Mailpit, esegue format/lint/tipi/build, Go vet/test con race detector e il ciclo Playwright completo. `.github/workflows/ci.yml` serve per pubblicare questa directory come repository autonomo; `../.github/workflows/radura-ci.yml` è il workflow nella root Git effettiva della cartella contenitore `twitter revival`, con working-directory e cache configurate per il progetto annidato. L’esecuzione remota richiede pubblicare il repository e non viene dichiarata verificata localmente.

### Test dello storage S3

Per eseguire anche l'integrazione media reale, avviare `docker compose up -d --wait db storage`, creare una volta il database dedicato `radura_test` e impostare nella shell dei test:

```sh
export TEST_DATABASE_URL='postgres://radura:LOCAL_PASSWORD@localhost:5432/radura_test?sslmode=disable'
export TEST_S3_ENDPOINT='http://127.0.0.1:8333'
export S3_ACCESS_KEY='radura-local'
export S3_SECRET_KEY='SAME_RANDOM_LOCAL_SECRET_AS_STORAGE'
cd services/api
go test -count=1 -v ./...
```

I test creano/rimuovono schemi isolati e usano il bucket separato `radura-tests`, con pulizia degli oggetti creati. Senza `TEST_S3_ENDPOINT` i test media reali sono esplicitamente saltati; senza `TEST_DATABASE_URL` viene saltata l'integrazione PostgreSQL. Non usare database/bucket di produzione. PowerShell: `$env:TEST_S3_ENDPOINT='http://127.0.0.1:8333'` e analogamente le altre variabili. Go non carica automaticamente `.env`.

La suite Playwright comprende upload S3, rifiuto SVG mascherato, recupero bozza, ricerca/notifiche e blocchi, più report, decisioni/ripristino/audit, ZIP con immagini e cancellazione account. Il polling è provato accelerando solo l'orologio browser; API/DB/S3 reali. La prova moderatore crea un proprio account, assegna/revoca il ruolo con CLI locale; non tocca report preesistenti. In Compose usa `docker compose exec`; per API native impostare `E2E_API_CLI` al binario compilato (CI: `services/api/server`) con `DATABASE_URL` appropriato. Non avviare backup/pausa API durante gli E2E. Screenshot nelle tre viewport; fixture PNG originale. Le esportazioni nei risultati E2E contengono esclusivamente dati sintetici del test.

Entrambi i workflow CI avviano anche SeaweedFS 4.48 e impostano `TEST_S3_ENDPOINT`, con credenziali esclusivamente locali di test. Il workflow nella root Git effettiva resta `../.github/workflows/radura-ci.yml`; il template interno serve per un repository autonomo. Pipeline remota non ancora eseguita.

## Struttura essenziale

```text
apps/web/src/app/                 pagine App Router e design token
apps/web/src/components/          layout, auth, anteprima anonima demo
apps/web/src/components/posts/    composer, card, feed e conversazioni reali
apps/web/src/components/profile/  profilo, editor, connessioni, blocchi, persone
apps/web/src/components/discovery/ ricerca e notifiche
apps/web/src/components/safety/   report, moderazione e gestione dati
apps/web/tests/                   smoke, auth, identità, post e feed reali
services/api/cmd/server/          API e comandi migrazione/healthcheck
services/api/internal/auth/       servizio e repository PostgreSQL
services/api/internal/users/      profili e grafo sociale
services/api/internal/posts/      CRUD, interazioni, feed e visibilità
services/api/internal/media/      validazione, ricodifica, repository e S3
services/api/internal/search/     validazione query e pattern letterali
services/api/internal/notifications/ elenco, conteggi e lettura
services/api/internal/moderation/ report, decisioni, audit e CLI ruoli
services/api/internal/privacy/    ZIP, cancellazione e purge durevole
services/api/internal/safety/     lock account e quote persistenti
services/api/internal/backup/     manifest SHA-256 di tabelle e immagini
services/api/internal/mailer/     invio SMTP con timeout e STARTTLS
services/api/internal/httpapi/    handler, CSRF, rate limit, test
services/api/internal/db/         query Go generate da sqlc
services/api/db/queries/          SQL sorgente
services/api/migrations/          schema, rollback, runner embedded
docs/                            architettura, provenienza UI, stato verifiche
.github/workflows/ci.yml          pipeline
compose.yaml                     db/storage/mailpit → migrate → api → web
infra/restore.compose.yaml       copia isolata per verifica restore
scripts/backup-verify.ps1         snapshot, restore e verifica locale
```

Esiti e file dello Sprint 5 nel [registro](docs/implementation-status.md), politiche nell'[architettura Sprint 5](docs/architecture/sprint-5.md). Il prossimo incremento è Sprint 6 (beta a inviti), solo su richiesta: nessun deploy, servizio a pagamento o funzione futura presentata come pronta.
