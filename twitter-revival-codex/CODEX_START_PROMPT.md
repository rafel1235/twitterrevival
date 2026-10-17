# Prompt da incollare in Codex

Sei il lead full-stack engineer del progetto **Twitter Revival**, un social network indipendente ispirato all'esperienza Twitter Web del 2021. Devi **scrivere e verificare codice reale nel repository**, non limitarti a proporre un piano.

Leggi interamente i file `AGENTS.md` e `PROJECT_SPEC.md` presenti nella root: contengono la specifica completa, l'architettura, i vincoli di sicurezza e proprietà intellettuale, il modello dati, le API e la roadmap. Se non li trovi, segnalalo prima di procedere.

## Compito immediato: implementa SOLO Sprint 1

1. Esamina i file e lo stato di git; preserva il lavoro preesistente.
2. Crea/configura un monorepo `apps/web` (Next.js App Router, React, TypeScript), `services/api` (Go + Chi) e PostgreSQL con `compose.yaml`.
3. Crea migrazioni PostgreSQL per `users`, `profiles`, `sessions` e relative constraint.
4. Implementa API reali `GET /healthz`, `POST /api/v1/auth/register`, `POST /api/v1/auth/login`, `POST /api/v1/auth/logout`, `GET /api/v1/auth/me`. Argon2id, token sessione CSPRNG hashato nel DB, cookie HttpOnly, controlli CSRF/origin per mutazioni, input validation e status code corretti.
5. Configura proxy same-origin `/api/v1` dal frontend alla API e forme di registrazione/login realmente collegate al backend.
6. Realizza un frontend responsive ispirato al Twitter Web 2021: sidebar, top bar, colonna feed di circa 600px, post card, composer, barra laterale desktop e mobile. Usa brand placeholder **originale**: niente logo o nome ufficiale Twitter/X mostrati come se fosse una piattaforma affiliata.
7. Finché non esiste il backend dei post, mostra un feed demo **evidentemente etichettato** e un composer non ingannevole (disabilitato o con indicazione "disponibile in Sprint 3"). Niente tweet salvati solo in stato client spacciati per post veri.
8. Prepara `.env.example`, README con comandi riproducibili, test backend e almeno uno smoke test Playwright; aggiungi GitHub Actions se ragionevole nel tempo del primo sprint.
9. Esegui format, lint, test, build e avvio smoke quando l'ambiente lo consente; se Docker o altri tool non sono disponibili, documenta ciò che non hai potuto eseguire.
10. Aggiorna `docs/implementation-status.md` con file creati, attività completate, test/esiti, aspetti incompleti e prossimi passi.

**Vincoli fondamentali:** Non implementare subito Sprint 2–6, DM, ActivityPub, microservizi o cloud deployment. Non fare scraping di X, non copiare codice o asset proprietari. Non creare servizi a pagamento o effettuare deploy senza richiesta. Preferisci una base completa e testata a uno scheletro enorme non funzionante.

**Output finale desiderato:** breve panoramica delle scelte tecniche, albero file essenziale, comandi per eseguire in locale, elenco degli endpoint funzionanti, test eseguiti con risultati reali, limitazioni note e prossimo passo operativo.

Inizia ora dall'ispezione del repository e procedi all'implementazione dello Sprint 1.
