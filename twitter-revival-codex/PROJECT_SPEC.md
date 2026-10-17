# Twitter Revival — Specifica di prodotto e architettura

**Versione:** 1.0 · **Data:** 17 ottobre 2026  
**Lingua di lavoro:** italiano per documentazione e comunicazioni, inglese per identificatori nel codice.

## 1. Visione e vincoli

Creare da zero un social network web indipendente che recuperi l'esperienza d'uso del Twitter classico (riferimento soprattutto alla UI web 2021): feed cronologico, interfaccia compatta, post brevi, profili, follow, like, repost, risposte, notifiche. Deve essere una piattaforma reale e utilizzabile, non soltanto una replica estetica, demo statica o wrapper del sito X.

**Distinzione legale essenziale:** `Twitter Revival` è un nome di progetto interno/provvisorio, **non** il nome commerciale da usare in produzione senza opportune verifiche. Nessuna affermazione di affiliazione a Twitter/X. Non copiare codice sorgente, asset proprietari, loghi originali, marchi, icone proprietarie, fotografie o dati utenti. Usare icone originali o open source con licenze compatibili. La UI può **ispirarsi** a layout e interazioni storiche; evitare una presentazione ingannevole dell'origine del servizio. Colore storico `#1DA1F2` solo come riferimento cromatico, non autorizzazione al marchio.

### Obiettivi qualitativi
- UI desktop a tre colonne (sidebar, feed centrale ~600 px, sidebar trend/ricerca) e versione mobile responsiva.
- Feed *Following* cronologico, senza ranking algoritmo nella prima versione.
- Accessibilità da tastiera, focus visibile, etichette ARIA appropriate, contrasto adeguato.
- API REST documentata, tipi generati dal contratto OpenAPI quando introdotti.
- Sicurezza di base fin dal primo sprint, moderazione prima della beta pubblica.
- GDPR: minimizzazione, esportazione e cancellazione dati, policy e verifica giuridica prima del lancio pubblico.

### Non obiettivi iniziali
- Non clonare X o utilizzare API private / scraping autenticato di X.
- Non importare account, follower o contenuti da X senza basi giuridiche/autorizzazioni.
- Non usare microservizi, Kubernetes, Kafka, Elasticsearch o sistemi complessi in anticipo.
- Non sviluppare contemporaneamente app native, federazione ActivityPub, Spaces, sistemi ads o timeline algoritmiche.

## 2. Stack di riferimento

| Strato | Tecnologia | Note |
|---|---|---|
| Frontend | Next.js App Router + React + TypeScript | CSS Modules + Tailwind; niente UI kit che ostacoli la fedeltà del layout |
| Backend | Go + Chi | Monolite modulare, handler → service → repository |
| SQL | PostgreSQL + pgx + sqlc | Migrazioni versionate, query parametrizzate |
| Cache e queue | Redis + Asynq (fase successiva) | Non indispensabili per avviare lo Sprint 1 |
| Media | Storage compatibile S3 (es. R2) | In locale eventuale mock/MinIO in sprint dedicato |
| Ricerca | PostgreSQL full-text search | Servizio separato solo se misurazioni lo richiedono |
| Realtime | SSE per notifiche (fase successiva) | Polling inizialmente accettabile |
| Deploy | Docker / Compose, CI GitHub Actions | Ambienti dev e prod distinti |
| Osservabilità | Structured logs, healthcheck, metriche | OpenTelemetry/Grafana quando serve |
| Test | Go test, Vitest/RTL, Playwright | E2E reali e deterministici |

Usare versioni stabili e compatibili disponibili nell'ambiente, senza bloccare il progetto su numeri di versione ipotizzati. Registrare la versione effettiva nei lockfile e in README.

## 3. Architettura logica

```text
Browser (Next.js, same origin)
        |
        v
  /api/v1/*  --> reverse proxy/rewrites --> Go API (Chi)
                                           |--- auth
                                           |--- users/profiles
                                           |--- posts/threads
                                           |--- social/follows
                                           |--- feed
                                           |--- interactions
                                           |--- notifications
                                           |--- moderation
                                           |--- media
                                           v
                                      PostgreSQL
                                           |
                                           +--- Redis/worker (solo quando necessario)
                                           +--- object storage (sprint media)
```

Il frontend si collega alle API tramite path relativi `/api/v1/` sulla stessa origine. In sviluppo usare una rewrite Next.js verso il servizio Go per evitare problemi CORS e gestire cookie con semplicità. In produzione configurare proxy equivalente. L'API non deve fidarsi delle verifiche effettuate dal frontend.

## 4. Modello dati progettuale

1. `users(id UUID PK, handle CITEXT UNIQUE, email CITEXT UNIQUE, password_hash TEXT, email_verified_at, created_at, updated_at, status)`
2. `profiles(user_id PK/FK, display_name, bio, avatar_key, banner_key, location_text, website_url)`
3. `sessions(id UUID PK, user_id FK, token_hash UNIQUE, expires_at, revoked_at, created_at)`
4. `posts(id UUID PK, author_id FK, body TEXT, reply_to_id nullable FK, quote_of_id nullable FK, visibility, created_at, deleted_at)`
5. `follows(follower_id FK, followed_id FK, created_at, PK(follower_id, followed_id))`
6. `likes(user_id FK, post_id FK, created_at, PK(user_id, post_id))`
7. `reposts(user_id FK, post_id FK, created_at, PK(user_id, post_id))`
8. `media(id UUID PK, owner_id FK, storage_key, mime_type, bytes, created_at)`
9. `post_media(post_id FK, media_id FK, position, PK(post_id,media_id))`
10. `notifications(id UUID PK, recipient_id FK, actor_id FK, type, post_id nullable, created_at, read_at)`
11. `blocks(blocker_id FK, blocked_id FK, created_at, PK(blocker_id,blocked_id))`
12. `reports(id UUID PK, reporter_id FK, target_type, target_id, reason, status, created_at, reviewed_at)`
13. `moderation_actions(id UUID PK, moderator_id FK, target_type, target_id, action, reason, created_at)`

**Implementare in maniera incrementale:** Sprint 1 può limitarsi a `users`, `profiles`, `sessions` e migrazioni iniziali; aggiungere le altre tabelle negli sprint dedicati. Non creare modelli fittizi per funzioni non implementate.

Vincoli: unicità, FK, check su self-follow, indici su `posts(author_id,created_at,id)`, `follows(followed_id)`; utilizzare paginazione a cursore; transazioni per operazioni su più tabelle. Date UTC lato storage, date locali lato interfaccia.

## 5. Regole prodotto

- Handle univoci e case-insensitive, es. 3–20 caratteri `[A-Za-z0-9_]` (policy provvisoria, documentata).
- Post testuali max 280 **code point Unicode** nella prima versione, da validare sia sul server che sul client; non fingere compatibilità perfetta con l'algoritmo di conteggio di Twitter.
- Contenuto utente sempre trattato come non attendibile; escaping output; niente rendering HTML arbitrario in tweet.
- Feed home: post degli account seguiti (ed eventualmente propri), ordinati `created_at DESC, id DESC`, paginazione a cursore; verificare visibilità, blocchi e moderazione.
- Like, follow, repost idempotenti con vincoli unici; distinguerli dalla citazione (nuovo post con `quote_of_id`).
- Cancellazione post con soft delete e policy di retention adeguata, senza garantire che i contenuti siano cancellati da backup istantaneamente.
- Notifiche generate solo per azioni consentite; non notificare automaticamente l'utente per ogni sua azione.

## 6. API HTTP target (progressiva)

```text
GET    /healthz
POST   /api/v1/auth/register
POST   /api/v1/auth/login
POST   /api/v1/auth/logout
GET    /api/v1/auth/me
GET    /api/v1/users/{handle}
PATCH  /api/v1/users/me
POST   /api/v1/users/{id}/follow
DELETE /api/v1/users/{id}/follow
POST   /api/v1/users/{id}/block
DELETE /api/v1/users/{id}/block
POST   /api/v1/posts
GET    /api/v1/posts/{id}
DELETE /api/v1/posts/{id}
POST   /api/v1/posts/{id}/likes
DELETE /api/v1/posts/{id}/likes
POST   /api/v1/posts/{id}/reposts
DELETE /api/v1/posts/{id}/reposts
GET    /api/v1/feed/home?cursor=...&limit=...
GET    /api/v1/search?q=...
GET    /api/v1/notifications?cursor=...
POST   /api/v1/reports
```

JSON consistente: envelope documentato per successo/errori, errori senza fuga di stack trace; status code appropriati; input validation. I client non devono dipendere da campi non documentati.

### Autenticazione / sicurezza
- Password con Argon2id usando parametri documentati e testati; nessuna password nei log.
- Token di sessione opachi generati con CSPRNG, memorizzare in DB **solo hash**; cookie HttpOnly, Secure in produzione, SameSite=Lax e Path=/.
- In sviluppo su `http://localhost` cookie Secure=false solo per ambiente dev.
- Origin/Referer validation e difesa CSRF per tutte le mutazioni basate su cookie (non considerare SameSite sufficiente).
- Controlli di autorizzazione lato server su ogni entità, rate limiting su auth e scrittura, limiti dimensione richieste.
- Segreti solo in variabili d'ambiente non committate; `.env.example` senza valori veri.
- Moderazione prima della beta pubblica: report, blocchi, sospensioni, pannello moderatori con audit.

## 7. Reverse engineering UI: approccio lecito e riproducibile

**Riferimento principale:** Twitter Web circa 2021; raccogliere esempi visivi da pagine pubbliche storiche, screenshot e archivio web. Nessun recupero di codice proprietario e nessuna dipendenza da server X.

Realizzare un inventario delle schermate:
- Home (feed, composer, navigazione tre colonne)
- Login / registrazione
- Profilo personale e altri profili
- Dettaglio post e thread
- Ricerca
- Notifiche
- Impostazioni essenziali

Creare in `docs/reverse-engineering/` una matrice `screen → elementi UI → stati → responsive → riferimento/provenienza → implementazione`. Distinguere fatti osservati e ipotesi. Per ogni componente definire variante default/hover/focus/loading/empty/error.

Design token iniziali ispirati a Twitter storico (soggetti a verifica):
```css
--color-primary: #1da1f2;
--color-text: #14171a;
--color-muted: #657786;
--color-border: #e1e8ed;
--color-background: #ffffff;
--feed-width: 600px;
```

**Non usare il logo originale del volatile** nelle schermate destinate a pubblicazione. Per sviluppo e test usare un marchio placeholder originale, neutro, facilmente sostituibile. Il titolo visibile non deve suggerire ufficialità o affiliazione.

Componenti target:
`AppShell`, `LeftNavigation`, `RightSidebar`, `Timeline`, `PostCard`, `PostComposer`, `PostActions`, `UserAvatar`, `ProfileHeader`, `FollowButton`, `ThreadView`, `AuthForm`, `SearchInput`, `NotificationItem`, `EmptyState`, `LoadingSkeleton`, `ErrorState`.

Verifica con screenshot Playwright a viewport desktop (es. 1440×900), tablet (768×1024) e mobile (390×844). La comparazione visuale serve a mantenere coerenza, non a copiare asset.

## 8. Roadmap in 6 fasi

### Sprint 1 — Settimane 1–2: fondamenta + prima vertical slice
- Monorepo: `apps/web`, `services/api`, `infra`, `docs`.
- Frontend Next.js e API Go avviabili con un comando documentato.
- PostgreSQL in Compose, schema e migrazioni versionate.
- Home/UI classica con contenuti demo esplicitamente identificati come demo; sidebar, post card, composer e responsive.
- Registrazione, login, logout e `me` effettivamente connessi al database, non mock.
- Test backend auth, smoke test Playwright; README e `.env.example`.

### Sprint 2 — Settimane 3–4: identità e grafo sociale
- Profili e avatar placeholder, modifica profilo, follow/unfollow, blocchi.
- Verifica email e recupero password (se non pronti nello Sprint 1).
- Vincoli e test delle autorizzazioni.

### Sprint 3 — Settimane 5–6: post e timeline reale
- CRUD essenziale dei post, risposte, like, repost, citazioni.
- Following feed paginato a cursore, ordinato cronologicamente.
- Test query e comportamento di account bloccati.

### Sprint 4 — Settimane 7–8: media, ricerca e notifiche
- Upload immagini con controlli, storage S3-compatible.
- Ricerca PostgreSQL FTS e ricerca utenti.
- Notifiche persistite e polling o SSE.
- UI mobile stabile.

### Sprint 5 — Settimane 9–10: sicurezza, GDPR e moderazione
- Report, blocchi, pannello moderatore e audit.
- Rate limit, controllo spam, log sicuri, cancellazione/esportazione dati.
- Backup e restore verificati, review dei rischi principali.
- Documentazione privacy/DSA da sottoporre a consulenza legale prima del lancio.

### Sprint 6 — Settimane 11–12: beta a inviti
- E2E completo, load test, remediation, monitoring, deploy con HTTPS.
- Avvio controllato con ~30–50 utenti pilota.
- Misurare attivazione, primi follow, prime interazioni, retention D7.

**Nota:** le durate sono obiettivi, non promesse. Prima della beta è indispensabile verificare sicurezza, moderazione, licenze e adempimenti applicabili.

## 9. MVP: in e fuori scope

**Nel MVP:** registrazione/login, profili, timeline cronologica, post testuali, follow, like, reply, repost, quote, blocchi, segnalazioni, moderazione di base, responsive, ricerca essenziale, immagini, notifiche di base.

**Fuori MVP:** messaggistica DM, GIF/video avanzati, trending sofisticati, liste, segnalibri, OAuth pubblico, ActivityPub, mobile nativo, Spaces/Community.

### Criterio end-to-end del MVP
Due utenti reali si registrano, fanno login, si seguono, pubblicano post, vedono la propria timeline, rispondono, mettono like e repost, si bloccano; un utente può segnalare e un moderatore può gestire la segnalazione; i dati persistono dopo riavvio. Nessuna funzionalità indicata come implementata deve essere un falso pulsante.

## 10. Repository desiderato

```text
twitter-revival/
├── AGENTS.md
├── README.md
├── .env.example
├── compose.yaml
├── apps/
│   └── web/
│       ├── src/app/
│       ├── src/components/{layout,timeline,posts,profile,auth}/
│       ├── src/lib/
│       └── tests/
├── services/
│   └── api/
│       ├── cmd/server/
│       ├── internal/{auth,users,posts,feed,social,moderation}/
│       ├── db/queries/
│       └── migrations/
├── packages/
│   └── api-contract/
├── infra/
│   └── monitoring/
├── docs/
│   ├── architecture/
│   ├── reverse-engineering/
│   └── design-system/
└── .github/workflows/
```

La struttura è una direzione, non un vincolo a creare directory vuote. Preferire file esistenti con responsabilità chiara.

## 11. Definition of Done comune

Ogni incremento deve:
1. Compilare e avviarsi tramite comandi documentati.
2. Includere migrazioni e rollback o strategia verificata per cambiare schema.
3. Avere test pertinenti automatici e nessun errore noto nascosto.
4. Rispettare confini di autenticazione e autorizzazione.
5. Avere stato UI per loading, empty, error e responsive.
6. Non contenere dati/credenziali veri o asset non autorizzati.
7. Aggiornare README e un breve `docs/implementation-status.md` con completato, incompleto, comandi provati e prossimi passi.

## 12. Rischi principali

- **Marchio e asset:** nome e logo storico potrebbero restare tutelati; scegliere brand nuovo prima del pubblico.
- **Effetto rete:** partire da una community piccola con utenti pilota, non da un lancio indiscriminato.
- **Abusi:** spam, harassment, contenuti illegali, bot: introdurre controlli e governance prima della beta.
- **Tempi:** un social production-grade richiede più di una UI convincente; ridurre scope anziché fingere implementazioni.
- **Scalabilità:** niente soglie di utenti promesse; misurare DAU, fan-out, query e costi reali.

## 13. Indicazioni a Codex

Leggere `AGENTS.md` e questo documento. Implementare **un singolo sprint per volta** con codice reale e test. Se ci sono ambiguità non bloccanti, scegliere soluzioni semplici e documentare le assunzioni. Non modificare codice fuori scope senza necessità. A fine sprint dare elenco file principali, comandi eseguiti, esito dei test, limitazioni e successivo sprint raccomandato.
