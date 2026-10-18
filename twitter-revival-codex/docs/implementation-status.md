# Stato implementazione — Sprint 8

Data: **18 ottobre 2026**. Prodotto **Radura**. Ambito autorizzato: **community pubbliche con iscrizione libera e contenuti riservati ai membri**, nella beta locale. La specifica originale arriva allo Sprint 6 e colloca le community fuori dal primo MVP; la richiesta esplicita dell’utente estende ora il prodotto. [Criteri e architettura](architecture/sprint-8.md).

Dominio, hosting e provider gestiti restano rimandati. Nessun acquisto, account esterno o deploy. Stato precedente conservato in [Sprint 7](implementation-status-sprint-7.md).

## Stato iniziale e conservazione

Letti integralmente AGENTS.md e PROJECT_SPEC.md. Git pulito su `main`, commit `c0cc37d sprint7`. Nessun reset, cambio ramo, commit o push. Nessuna modifica ai segreti locali, nessuna ricreazione dei volumi applicativi.

## Funzioni implementate

- Directory `/communities` con tutte/le mie, creazione e paginazione; dettaglio `/communities/{id}` con descrizione, regole e membri.
- Iscrizione/uscita idempotenti; creatore automaticamente membro e proprietario. Massimo 10 community attive per proprietario. Modifica dei metadati riservata al proprietario; archiviazione da proprietario o moderatore del sito con conferma UI. Il proprietario non può abbandonare una community attiva.
- Feed cronologico paginato e composer per soli membri. Post, immagini, risposte e like persistono nel backend esistente. Risposte con community ereditata sul server; audience immutabile. Repost e citazioni dei post di community respinti sul server.
- Metadati e lista membri scopribili nella beta autenticata; contenuti assenti da home, profili e ricerca globale. Letture dirette, immagini, riferimenti e notifiche protetti dalla stessa view di visibilità, con blocchi e moderazione.
- Lock account ordinati → community → coppia utenti → post → media: iscrizione, uscita e archiviazione serializzate con le scritture. Nuovi endpoint con autenticazione, CSRF/origin, input validation, rate limit e JSON coerente.
- Esportazione con proprie community/iscrizioni/eventi e `community_id` nei propri post. Cancellazione account rimuove iscrizioni, archivia/anonimizza le community possedute e scollega l’attore dagli eventi. Gli altri autori mantengono i propri post nell’esportazione, senza renderli pubblici.
- Migrazione **007**, indici e vincoli. Il rollback rifiuta atomicamente la presenza di qualunque post di community, anche eliminato/archiviato, per evitare ampliamenti di visibilità.
- UI responsive, navigazione dedicata, stati loading/empty/error, focus form, conferme di uscita/archiviazione e segnalazione del proprietario per abusi nei metadati. Nessuna nuova dipendenza di produzione o sviluppo.

## File principali

```text
services/api/migrations/007_communities.{up,down}.sql
services/api/internal/communities/{service.go,service_test.go}
services/api/internal/httpapi/{communities.go,sprint8_integration_test.go,communities_limits_test.go,communities_concurrency_test.go}
services/api/internal/{posts,privacy}/
services/api/db/queries/{posts,discovery,safety}.sql
services/api/internal/db/                     # rigenerato con sqlc 1.31.1
apps/web/src/app/communities/{page.tsx,[id]/page.tsx}
apps/web/src/components/communities/
apps/web/src/lib/communities.ts
apps/web/tests/{communities.spec.ts,community-layout.spec.ts,community-fixtures.ts}
docs/{architecture,reverse-engineering,security}/sprint-8*.md
```

Aggiornati anche wiring API, versione migrazioni/sqlc, composer/card/dettaglio post, navigazione e icona originale, informativa locale `/privacy` e avviso nella cancellazione account, README e test rollback Sprint 6 per arrivare alla versione corretta.

## Verifiche effettive

| Controllo                                           | Esito                                                                                                                                                             |
| --------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Unit Go posts/community su Windows                  | Passati; community: 3 test + 8 sottocasi                                                                                                                          |
| Backend completo su DB/S3 effimeri                  | **59 test + 41 sottocasi passati**, nessun fallito o saltato; gofmt/vet/build passati; progetto `radura-backend-check-b6938b64` rimosso                           |
| Lint / TypeScript / unit client API                 | Passati; unit client API 10/10                                                                                                                                    |
| Build immagini API/migrazioni/frontend              | Passate, inclusa build finale con correzioni CSS e avviso nella cancellazione account                                                                             |
| Migrazione beta corrente e avvio                    | Passati: schema 6 → 7, invariati 424 utenti / 498 post / 338 sessioni prima dei test browser; 7 servizi sani                                                      |
| Community e accessibilità dedicate                  | **12/12 passati** su desktop/tablet/mobile, inclusi 320px, testo 200% e nomi Unicode di 60 caratteri; screenshot ispezionati                                      |
| Regressione browser completa                        | **95/96 passati al primo giro** (8,4 minuti), zero saltati; un errore di sincronizzazione nel test ricerca desktop, corretto                                      |
| Ricerca/media/notifiche dopo la correzione del test | **3/3 passati** su desktop/tablet/mobile (1,1 minuti), zero saltati; output separato. La suite da 96 non ripetuta integralmente dopo questa sola modifica al test |
| HTTPS finale                                        | **3/3 passati**: catena CA locale verificata, redirect, cookie Secure/HttpOnly, sessione e Origin ostile                                                          |
| Audit produzione                                    | **0 vulnerabilità note**, audit npm rieseguito; nessuna nuova dipendenza                                                                                          |
| Formattazione / diff                                | `npm run format:check` e `git diff --check` passati; resoconto finale formattato e ricontrollato                                                                  |

I test backend usano un progetto Docker casuale, senza porte host, con password sintetiche e DB/storage effimeri. Nessuna manutenzione o riconciliazione sugli oggetti della beta. I nuovi test browser utilizzano account sintetici delle coorti di collaudo, rimossi tramite la propria password. Guasti simulati del browser sono distinti dalle letture/scritture reali.

Nel primo collaudo browser delle community sono emersi due errori axe `target-size` sui link autore/data; aumentata l’area cliccabile a 24 px. La prova con nome lungo e testo al 200% ha rilevato il sottotitolo del dettaglio tagliato: header reso adattabile. La verifica geometrica iniziale includeva anche una label `.sr-only`, intenzionalmente nascosta; esclusa solo dalla geometria visiva, mantenendo l’audit axe integrale. I 12 controlli dedicati successivi sono passati. La regressione preesistente ha inoltre esposto una corsa nel test della ricerca: inseriva il testo nel form precedente al completamento del cambio Post/Persone. Il test attende ora il filtro effettivamente montato e la query nell’URL, senza pause artificiali o timeout aumentati; codice di ricerca invariato.

Artefatti locali ignorati da Git: `.local/backend-tests/radura-backend-check-b6938b64/go-test.jsonl`, `.local/sprint8-migration-{before,after}.json`, `.local/npm-audit-sprint8-production.json`, `.local/sprint8-communities-final/`, `.local/playwright-sprint8-results.json` (primo giro completo), `.local/playwright-sprint8-discovery-results.json` e `.local/sprint8-discovery-final/` (ricerca dopo correzione), `playwright-report/` e `playwright-report-https/`. Screenshot di desktop, mobile e testo al 200% ispezionati.

## Limiti e prossimo passo

- “Pubbliche” significa scopribili dagli account della beta, senza link di invito al gruppo. Chiunque nella beta può iscriversi; non sono gruppi confidenziali a inviti. Nome, regole e membri sono visibili prima dell’iscrizione.
- I moderatori del sito possono esaminare le evidenze dei post segnalati; il normale dettaglio post richiede comunque iscrizione. Per problemi con i metadati si usa la segnalazione del proprietario, indicando la community. Nessuna nuova coda di segnalazioni community.
- Nessun trasferimento proprietà, ripristino UI, ruoli di moderatore di gruppo, espulsioni, inviti privati, chat o notifiche di iscrizione in questo incremento.
- L’uscita non elimina i vecchi post; il ritorno al gruppo ripristina l’accesso. Non si possono ritirare copie di contenuti già letti. Se il proprietario è sospeso il gruppo risulta non disponibile; l’esportazione e la cancellazione account restano le vie per gestire i propri dati.
- Nessuna certificazione WCAG o prova umana con screen reader; nessuna misura di utenti pilota o retention inventata. Audit dipendenze completo dello Sprint 7 con cinque segnalazioni dev note, nessuna nuova dipendenza in Sprint 8.
- Race detector, carico e restore non rieseguiti nello Sprint 8. Il backup enumera automaticamente le nuove tabelle; ciò non equivale a una nuova prova di ripristino. La CI esistente include automaticamente i nuovi test, ma non è stata eseguita su GitHub in questa sessione.

Avvio dalla root applicativa: `./scripts/start-local-beta.ps1 -Build` per aggiornare, poi `./scripts/start-local-beta.ps1`. Sito: **https://localhost:3443**, pagina **/communities**. Prossimo passo operativo: provare con due account iscrizione, post e uscita, poi raccogliere feedback prima di ampliare la gestione dei gruppi.
