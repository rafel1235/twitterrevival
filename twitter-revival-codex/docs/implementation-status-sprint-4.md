# Stato implementazione · 18 ottobre 2026

**Sprint 4 completato e verificato localmente: immagini su storage S3, ricerca PostgreSQL e notifiche persistenti.** Stack aggiornato attivo in Docker. Conservati i resoconti [Sprint 1](implementation-status-sprint-1.md), [Sprint 2](implementation-status-sprint-2.md) e [Sprint 3](implementation-status-sprint-3.md). Nessuna implementazione anticipata degli Sprint 5–6.

## Preservazione

Letti interamente `AGENTS.md` e `PROJECT_SPEC.md`; invariati insieme a `CODEX_START_PROMPT.md`. Git iniziale pulito su `main`, root nella cartella contenitore `twitter revival`. Nessuna modifica a indice, branch, cronologia o configurazione globale; nessun commit, push o deploy.

Backup PostgreSQL prima dell'upgrade: `.local/backups/before-sprint4.dump`, **59.140 byte**. Account, sessioni, grafo sociale, post, interazioni e volume PostgreSQL conservati. Aggiunte solo variabili S3 mancanti a `.env`, con un segreto CSPRNG distinto, senza stampare credenziali né sovrascrivere valori precedenti. Nuovo volume persistente `media_data`; nessun account cloud o servizio a pagamento. Segreti/backup/tool/artifact esclusi da Git.

## File e attività completate

| Area                | File principali                                                                                                                                                            | Risultato                                                                                 |
| ------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------- |
| Schema/query        | `services/api/migrations/004_discovery_media.{up,down}.sql`, `migrate.go`, `sqlc.yaml`, `db/queries/{media,discovery}.sql`, `internal/db/`                                 | media/post_media, FTS generato e GIN, notifiche con vincoli/trigger/view; sqlc rigenerato |
| Media               | `services/api/internal/media/{image,service,storage}.go`                                                                                                                   | Decode/ricodifica, MIME/dimensioni/quote, S3 privato, bozze, autorizzazione contenuti     |
| Ricerca/notifiche   | `internal/search/query.go`, `internal/users/search.go`, `internal/notifications/service.go`, modulo posts aggiornato                                                       | FTS post, ricerca handle/nome, cursori, notifiche atomiche e stato letto                  |
| API                 | `internal/httpapi/{media,discovery,server}.go`, `cmd/server/main.go`                                                                                                       | Route autenticate, multipart limitato, CSRF/origin, rate limit, startup/health S3         |
| Frontend            | `apps/web/src/app/{search,notifications}/`, `components/discovery/`, `components/posts/{attachments,draft-images,composer}.tsx`, `lib/notifications.ts`                    | Upload/preview/recupero bozze, ricerca, notifiche, badge/polling, cinque voci mobile      |
| Integrazione UI     | `post-card.tsx`, `post-list.tsx`, `connections.tsx`, `app-shell.tsx`, `globals.css`, `lib/{api,posts}.ts`                                                                  | Immagini e nuovi percorsi integrati alle funzioni esistenti; multipart same-origin        |
| Test                | `internal/media/image_test.go`, `internal/search/query_test.go`, `internal/httpapi/sprint4_integration_test.go`, `apps/web/tests/discovery.spec.ts`, fixture PNG originale | Test unit/integration/S3/browser; fixture precedenti adattate alla migrazione 004         |
| Configurazione/docs | `compose.yaml`, `.env.example`, `go.mod/go.sum`, entrambi i workflow CI, README, architettura/matrice Sprint 4 e questo registro                                           | Avvio riproducibile, storage persistente, endpoint e limiti documentati                   |

Unica nuova dipendenza applicativa: SDK AWS Go v2, **S3 1.114.3 / core 1.47.3 / credentials 1.20.9**, con dipendenze indirette fissate in go.sum. Nessuna nuova dipendenza npm; package-lock invariato. Nuovo servizio locale **SeaweedFS 4.48**, immagine ufficiale del progetto, endpoint S3 sul solo loopback 8333. PostgreSQL, Go/Chi, Next/React/TypeScript e Mailpit restano quelli verificati nello Sprint 3.

## Endpoint aggiunti

- `POST /api/v1/media`: file + alt_text multipart, 201 con metadati pubblici e URL same-origin.
- `GET /api/v1/media/drafts`: proprie bozze con scadenza e stato associabile.
- `GET /api/v1/media/{id}` e `GET /api/v1/media/{id}/content`: metadati/byte solo se visibili.
- `DELETE /api/v1/media/{id}`: 204 per bozza propria, 409 se associata, 404 se altrui/non disponibile.
- `GET /api/v1/search?q=…&type=posts|users&cursor=…&limit=…`.
- `GET /api/v1/notifications` e `GET /api/v1/notifications/unread-count`.
- `POST /api/v1/notifications/{id}/read` e `POST /api/v1/notifications/read`: 204, solo notifiche del destinatario.

`POST /posts` ora accetta `media_ids`, massimo quattro immagini proprie non riusate e caricate da meno di 24 ore. Le card/preview restituiscono `media[]`. Tutte le letture richiedono sessione; mutazioni protette da origin/Fetch Metadata/header CSRF. JSON limitato a 8 KiB, multipart a 5 MiB di file più overhead limitato. Rate limit nuovi: media 20 mutazioni/minuto, ricerca 120 letture/minuto, stato notifiche 60 mutazioni/minuto per peer.

## Scelte e controlli verificati

JPEG/PNG soltanto: max 5 MiB, 4096 px per lato, 8 megapixel; decode delle dimensioni prima dell'allocazione, poi ricodifica completa con rimozione metadati/byte aggiunti, output max 8 MiB. Descrizione obbligatoria 1–200 code point. Due elaborazioni simultanee, 12 bozze e 100 MiB attivi/account, verifica quota serializzata. Bucket privato, chiavi casuali a 256 bit, nessun URL S3/credenziale al browser; download con controllo visibilità, nosniff/CSP e private,no-store. Blocchi e post cancellati impediscono il download anche conoscendo l'URL.

Ricerca FTS PostgreSQL simple/websearch sui post, prefisso handle o sottostringa nome per persone, senza email. Indice aggiornato all'edit; wildcard LIKE letterali. Cursori e ordine cronologico; filtri su blocchi/account inattivi. Notifiche follow/like/repost/reply/quote nella stessa transazione dell'azione, deduplicate, senza notifiche verso sé stessi o backfill storico. Undo/cancellazione/blocco rimuovono quelle collegate; lettura singola/tutte persistente. Polling ogni 30 secondi con scheda visibile, abort e protezione dalle risposte obsolete.

## Verifiche realmente eseguite

| Verifica                                                                | Esito                                                                                                                               |
| ----------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------- |
| `sqlc generate`, `gofmt`, `go vet ./...`                                | PASS                                                                                                                                |
| `go test -count=1 -json ./...` con TEST_DATABASE_URL e TEST_S3_ENDPOINT | **30 test principali + 31 sotto-casi PASS, zero skip, exit 0**; PostgreSQL e S3 reali                                               |
| `go build -o … ./cmd/server`                                            | PASS nativo e nel build Docker                                                                                                      |
| `npm run format:check`, `npm run typecheck`, `npm run lint`             | PASS; zero warning/errori ESLint                                                                                                    |
| Build Next finale in Docker                                             | PASS, TypeScript e 13 pagine statiche generate; nuove route /search e /notifications                                                |
| `docker compose config --quiet` e `up --build -d --wait`                | PASS; db, storage, mailpit, api e web healthy, migrate Exited (0)                                                                   |
| Playwright completo con E2E_AUTH=1, E2E_MAILPIT=1 e base URL Docker     | **30/30 PASS, nessuno skip**; desktop/tablet/mobile, polling incluso                                                                |
| Riavvio storage/API/web                                                 | PASS: stessa sessione e post leggibili, hash SHA-256 dei byte immagine identico prima/dopo; probe rimosso                           |
| UI                                                                      | Screenshot a 1440×900, 768×1024 e 390×844; assenza overflow verificata, ricerca con immagine e notifiche desktop/mobile ispezionate |

Go copre rigetto SVG/MIME falso/dimensioni/descrizioni/file corrotti, ricodifica, bucket anonimo negato, bozze private, allegati non riusabili/scaduti, quota concorrente, FTS/cursori/edit/blocchi e notifiche idempotenti/atomiche/read. Rollback 004 conserva gli sprint precedenti; test distruttivi solo su schemi isolati nel database `radura_test`. S3 di test nel bucket separato `radura-tests` con pulizia oggetti. Log JSON del run Go in `.local/go-test-sprint4.jsonl`, ignorato da Git.

Playwright mantiene le regressioni precedenti e verifica upload/recupero bozza dopo refresh, naturale larghezza dell'immagine servita da S3, ricerca post/persone, cinque notifiche, stato letto dopo reload e blocchi su contenuti/ricerca/notifiche. Per il polling accelera soltanto l'orologio Chromium; HTTP, DB e storage restano reali. Nessun mock/intercettazione delle API. Report/screenshot in `playwright-report/` e `test-results/`, ignorati. Fixture paesaggio 600×360 originale generata proceduralmente, nessun asset proprietario.

Iterazioni risolte: qualifica delle colonne SQL nelle query lettura notifiche; guardia null TypeScript nel badge. Download SDK inizialmente impedito dal sandbox di rete, poi eseguito con autorizzazione automatica; Docker/Chromium usati fuori dal sandbox Windows. I limiti auth restano attivi e i test rispettano Retry-After. Nessun errore funzionale lasciato nascosto.

## Limiti e prossimo passo

- Niente media video/GIF/WebP o upload avatar/banner; avatar a iniziali. Testo post obbligatorio, allegati immutabili. Orientamento EXIF non interpretato.
- Nessun purge/retention o riconciliazione S3 durevole: crash e rollback possono lasciare oggetti orfani. Soft delete conserva i byte e la quota; predisporre retention e backup/restore di DB **e** storage prima della beta.
- FTS essenziale senza stemming/rimozione accenti/ranking; nessun load test. Quota per peer condivisa dietro Next e non distribuita.
- Polling, niente SSE/push/menzioni. Aggiornamento pagina notifiche dalla prima pagina, anche dopo caricamento di pagine precedenti.
- Restano i limiti SMTP dello Sprint 2: cattura locale, coda in memoria, verifica email non obbligatoria nell'anteprima.
- **CI GitHub Actions remota/race detector Linux non eseguiti.** Workflow estesi a SeaweedFS e integrazione S3; nessun push/deploy. Sicurezza operativa, GDPR, moderazione e audit restano allo Sprint 5.

Aprire <http://localhost:3000>, accedere e provare Aggiungi immagine, [Esplora](http://localhost:3000/search) e [Notifiche](http://localhost:3000/notifications). Mailpit resta su <http://localhost:8025>. Servizi lasciati attivi. Riavvio: `docker compose up --build -d --wait`; arresto senza perdita dati: `docker compose stop`. Non sovrascrivere `.env`.

Prossimo incremento: **Sprint 5, sicurezza/GDPR/moderazione**, su richiesta. Contratti/setup nel README, politiche nell'[architettura Sprint 4](architecture/sprint-4.md), provenienza/stati nella [matrice UI](reverse-engineering/sprint-4-matrix.md).
