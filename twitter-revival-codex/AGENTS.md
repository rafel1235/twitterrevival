# AGENTS.md — Istruzioni per Codex / agenti di coding

Questo repository implementa un social network indipendente ispirato all'esperienza del Twitter classico. La fonte autorevole dei requisiti è `PROJECT_SPEC.md` (quando presente nella root; se distribuito in `docs/`, aggiornare questo link).

## Principi non negoziabili

1. **Codice realmente funzionante:** mai sostituire backend reale con finti fetch, finti login, pulsanti inattivi o storage in-memory presentati come persistenti. I contenuti demo UI devono essere esplicitamente etichettati.
2. **Un incremento alla volta:** completare e verificare lo sprint richiesto prima di avviare feature future. Non promettere la realizzazione dell'intero prodotto in una sola iterazione.
3. **Architettura pragmatica:** Go/Chi modular monolith, PostgreSQL, Next.js/React/TypeScript, Compose. Redis e storage S3 solo quando richiesti dallo sprint.
4. **API coerenti:** prefisso `/api/v1`; same-origin in browser; `GET /healthz` per healthcheck. Frontend con path relativi e proxy verso API Go.
5. **Sicurezza:** password Argon2id, token sessione opachi con hash nel database, cookie HttpOnly (Secure in produzione), origine verificata per le mutazioni, autorizzazione sempre nel backend. Non committare segreti.
6. **Reverse engineering lecito:** riferimenti storici solo per struttura/comportamento. Non copiare sorgenti e asset di Twitter/X, né usare il logo storico come marchio della nuova piattaforma. Brand placeholder originale.
7. **Documentazione e verificabilità:** README con setup, variabili `.env.example`, migrazioni, comandi per test, limitazioni, e `docs/implementation-status.md` aggiornato.
8. **Qualità:** test Go, unit/component test React ove pertinenti, Playwright per il percorso utente. Niente `TODO` spacciati per implementazione.

## Convenzioni

- Nome di progetto interno: Twitter Revival. Nome di prodotto visibile: placeholder indipendente e configurabile.
- Lingua della documentazione: italiano. Nomi funzioni/variabili e commenti tecnici: inglese.
- Post: 280 code point Unicode massimi, limite identico backend/client; test su emoji e caratteri accentati.
- Handle: 3–20 ASCII lettere/cifre/underscore, univoco case-insensitive.
- UTC in database; rendering date locale.
- Errori API: JSON consistente, senza dettagli interni o segreti.
- Dipendenze stabili, lockfile committato, nessun pacchetto non necessario.
- Commits piccoli e descrittivi (se l'ambiente consente git); non riscrivere cronologia senza richiesta.

## Workflow obbligatorio

1. Esamina il repository esistente e leggi specifiche/istruzioni.
2. Descrivi brevemente l'incremento da eseguire e le assunzioni.
3. Implementa il codice e i test pertinenti.
4. Esegui, quando disponibili: format, lint, test, build, smoke test.
5. Riporta esattamente cosa hai verificato e cosa non puoi verificare nell'ambiente.
6. Aggiorna lo stato di implementazione.

Non creare account esterni, acquistare servizi, pubblicare su internet, registrare domini o attivare servizi a pagamento senza un'esplicita richiesta dell'utente.

## Scope del primo incarico (Sprint 1)

- Monorepo avviabile: Next.js + Go/Chi + PostgreSQL in Docker Compose.
- Schema e migrazioni per utenti, profili e sessioni.
- Registrazione, login, logout, endpoint `/api/v1/auth/me` con persistenza reale e test.
- UI responsive ispirata al layout Twitter 2021 con design token e marchio placeholder originale.
- Home con post demo chiaramente identificati finché il motore post non viene implementato nello Sprint 3.
- Nessuna funzione sociale avanzata finta presentata come pronta.
- Documentazione e test eseguibili.

**Criteri di completamento Sprint 1:**
- `docker compose up --build` avvia i servizi previsti, oppure fornire una variante documentata se tool non disponibili; non affermare che funziona senza eseguirla.
- Homepage visibile e adattabile al mobile.
- User A si registra; password è hashata; login crea una sessione persistita; refresh mantiene la sessione; logout revoca la sessione.
- Input errati restituiscono validazioni comprensibili; accesso a `/me` senza sessione dà 401.
- Test auth e almeno uno smoke test frontend sono presenti (eseguiti quando possibile).
