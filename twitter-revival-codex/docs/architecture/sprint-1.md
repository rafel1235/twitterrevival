# Architettura Sprint 1

```text
Browser → Next.js (3000) → rewrite /api/v1/* → Chi (8080) → PostgreSQL
                                               handler → auth.Service → PostgresRepository → sqlc/pgx
```

Next non gestisce credenziali o sessioni autonomamente: inoltra request e Set-Cookie. Il frontend usa `fetch` relativo e `credentials: same-origin`; stato React solo per presentare l’utente verificato, mai come prova di autenticazione. Al refresh legge `/auth/me` dal database.

Per questo incremento la UI usa CSS nativo e design token centralizzati, senza UI kit o framework CSS aggiuntivi. La specifica indica CSS Modules e Tailwind come stack di riferimento; in questa base il foglio globale è limitato alla shell e ai componenti dello Sprint 1. Non occorre introdurre dipendenze di stile per avviare l’applicazione.

Il servizio auth valida l’input, limita concorrenza Argon2, genera token e decide la durata. Il repository usa SQL parametrizzato generato da sqlc e transazioni per registrazione. Le autorizzazioni della sessione controllano stato utente, revoca e scadenza nel database. Logout revoca esclusivamente la sessione corrente; altre sessioni restano attive.

Migrazione in servizio Compose separato: nessuna gara di startup con la API, applicazione atomica e advisory lock. Il processo API richiede schema versione 1 presente prima di servire traffico. Healthcheck DB-aware; nessun Redis, worker, S3 o servizio futuro.

Scelte conservative: origin singolo configurato esplicitamente; header anti-CSRF e JSON obbligatori anche su logout; nessuna fiducia a forwarded IP; quota per peer condivisa dietro Next. Sessioni revocate/scadute vengono conservate: il job di pulizia e una retention motivata restano da definire prima della beta. Le migrazioni sono un runner minimo per la sola versione 1; aggiungere un catalogo sequenziale prima di introdurre la versione 2.

Le tabelle social e il limite post Unicode di 280 code point saranno implementati e testati nello Sprint 3. Non esiste alcun endpoint post né persistenza client di contenuti demo.
