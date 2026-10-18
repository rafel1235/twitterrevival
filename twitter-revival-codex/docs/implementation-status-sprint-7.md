# Stato implementazione — Sprint 7

Data: **18 ottobre 2026**. Prodotto **Radura**. Ambito scelto dall’utente: **usabilità, accessibilità e affidabilità della beta locale**. La roadmap originale termina allo Sprint 6; questo incremento è definito in [sprint-7.md](architecture/sprint-7.md).

**Dominio e hosting rimandati.** Nessun acquisto, account cloud, DNS, deploy o migrazione verso Vercel/Render/Supabase. Preparazione precedente conservata in [implementation-status-deployment-preparation.md](implementation-status-deployment-preparation.md); Sprint 6 in [implementation-status-sprint-6.md](implementation-status-sprint-6.md).

## Stato iniziale e conservazione

Letti AGENTS.md e PROJECT_SPEC.md, repository pulito su `main`, commit `7c8df05 sprint6 fix`. Nessun commit/push/reset/cambio ramo; specifiche e configurazione privata conservate. Nessuna migrazione SQL e nessun endpoint nuovo. Aggiornato soltanto il frontend del progetto locale; API, database, storage, casella e CA non ricreati.

## Attività implementate

- Nomi accessibili della navigazione tablet, label del brand coerente con il testo, contrasto dei testi secondari, pagina corrente desktop/mobile e skip link con focus reale nel main. Testo ingrandito al 200% senza tagli nel brand, nelle voci di navigazione e nel titolo laterale.
- Conteggi nei nomi delle azioni dei post, iniziali nei link avatar, riferimenti alle citazioni con nome derivato dal contenuto. Suggerimenti d’uso al posto delle milestone tecniche nell’interfaccia.
- Focus sul primo campo errato o sull’errore generale in accesso/registrazione, recupero password e modifica profilo. Errori collegati ai campi; stato di invio annunciato; valori conservati.
- API client con timeout letture 15s, mutazioni/upload 30s, export 60s, esteso al corpo della risposta; cancellazione distinta dal guasto di rete e parsing difensivo degli errori. Nessun retry automatico delle scritture; messaggio esplicito sull’esito incerto.
- Feed/notifiche conservati durante guasti transitori con avviso sugli ultimi dati; retry con il cursore fallito. 401/403/404 rimuovono i dati interessati. Liste connessioni con deduplica e protezione dalle risposte obsolete.
- Risorse annullate/superate durante aggiornamenti concorrenti; errore di verifica sessione distinto dalla demo anonima; avviso di sessione scaduta/revocata con link di accesso.
- Boundary per errori di rendering della singola pagina e recupero, shell preservata.
- Test unitari del client API e test Playwright con axe, tastiera, reflow 320px, testo 200%, errori e dati reali. Contesti dei test precedenti adattati all’HTTPS locale; verifica CA separata.
- Runner `npm run test:backend`: DB/S3 effimeri, password di test, nessuna porta host, progetto casuale, validazione topologia e cleanup delle sole risorse proprie.
- CI aggiornata con test unitari; la suite browser completa include automaticamente i nuovi test. Nessuna pipeline di deploy.

## File principali

```text
apps/web/src/lib/{api,use-resource,use-form-focus}.ts
apps/web/src/app/{error.tsx,layout.tsx,globals.css,theme.css}
apps/web/src/components/{auth,layout,posts,profile,discovery,timeline}/
apps/web/tests/{api,accessibility,resilience}.spec.ts
apps/web/tests/ux-fixtures.ts
playwright.unit.config.ts
infra/backend-tests.compose.yaml
scripts/test-backend.mjs
docs/architecture/sprint-7.md
docs/reverse-engineering/sprint-7-matrix.md
docs/security/sprint-7-review.md
```

Aggiornati inoltre README, package/lockfile, configurazione Playwright, test precedenti e workflow interno/root Git. Nuova dipendenza di sviluppo: `@axe-core/playwright` **4.14.0**; nessuna nuova dipendenza di produzione.

## Verifiche effettive

| Controllo                                 | Esito                                                                                                                                      |
| ----------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------ |
| Test client API in Node                   | **10/10 passati**: timeout, abort, corpo incompleto, JSON/HTML errati, cookie/CSRF, upload/blob, nessun retry                              |
| Backend isolato Linux                     | **45 test + 31 sottocasi passati**, nessun test saltato, gofmt/vet/build passati; progetto `radura-backend-check-3bed26d4` rimosso         |
| Lint / TypeScript                         | Passati                                                                                                                                    |
| Build Next di produzione in Docker        | Passata anche dopo l’ultima correzione CSS; frontend locale aggiornato e sano                                                              |
| Regressione browser completa              | **84/84 passati**, zero saltati o flaky, 4m26s; HTTPS locale, desktop/tablet/mobile, prima dell’ultima correzione CSS                      |
| Accessibilità sulla correzione CSS finale | **18/18 passati**, zero saltati, circa 1,3 minuti; axe, focus, contenitori del testo al 200% e reflow 320px; screenshot ispezionati        |
| Configurazione deployment conservata      | **9/9 passati**, parser/configurazione, sintassi Bash e gateway isolato: manutenzione 503, metriche 404, salute 200                        |
| HTTPS sulla build finale                  | **3/3 passati**, 8,9s; CA verificata, redirect, cookie Secure/HttpOnly, persistenza, Origin ostile respinto                                |
| Formattazione finale                      | `npm run format:check`, workflow nella root Git e `git diff --check` passati                                                               |
| Audit produzione                          | **0 vulnerabilità note**                                                                                                                   |
| Audit completo                            | Cinque segnalazioni high nella catena dev braces/ESLint, senza versione corretta indicata; [dettagli e fonte](security/sprint-7-review.md) |

Durante lo sviluppo, il primo controllo UX ha segnalato sei errori nelle etichette accessibili, poi corretti. Una regressione successiva ha rilevato il conteggio notifiche assente dal nome accessibile e una collisione degli artefatti di test: il runner unitario cancellava la directory usata contemporaneamente dalle trace browser. Conteggio corretto e directory unit/HTTPS separate; l’esecuzione completa successiva ha passato tutti gli 84 casi. L’ispezione visiva al 200% ha inoltre individuato testo tagliato che il solo controllo di overflow della pagina non rilevava: aggiunto un controllo dei singoli contenitori, verificato fallire sulla versione precedente e corretto il wrapping CSS.

Il primo comando backend non ha letto le variabili contenenti cifre e non ha passato il segreto S3: nove integrazioni media fallite per configurazione. La ripetizione sui servizi correnti è stata respinta dalla revisione automatica per il rischio dei test purge/orfani. È stato quindi creato e verificato il runner con DB/S3 completamente separati; il suo risultato completo è quello riportato sopra. Nessuna pulizia manuale dei dati correnti eseguita in questo sprint.

Artefatti privati, ignorati da Git: `.local/backend-tests/radura-backend-check-3bed26d4/go-test.jsonl`, `.local/playwright-sprint7-results.json`, audit npm, controlli e screenshot finali in `.local/sprint7-final-accessibility` e `.local/test-results-https`; report HTML in `playwright-report`/`playwright-report-https` e screenshot della regressione completa in `test-results`. Gli account di collaudo dei nuovi test sono sintetici e le fixture ne richiedono la cancellazione anche dopo un errore. I test di recupero iniettano guasti nel solo browser; i contenuti ordinari provengono da API/DB/S3 reali. Nessuna statistica di piloti umani inventata.

## Limiti e prossimo passo

- Nessuna certificazione WCAG: axe e prove tastiera coprono gli stati elencati nella matrice; collaudo umano con screen reader ancora da svolgere.
- La bozza resta nel componente durante errori recuperabili; navigazione, reload, perdita sessione o errore di rendering possono perderla. Nessun salvataggio privato nel browser.
- Timeout/cancellazione non dimostrano che una mutazione non sia stata applicata: verificare l’esito prima di reinviarla.
- Race detector, carico e backup/restore degli sprint precedenti non rieseguiti qui; codice/schema/backend invariati. Race nella CI Linux, esecuzione remota CI non dichiarata verificata.
- Beta ancora locale, senza piloti esterni o osservazione D7 reale. Hosting e dominio restano rinviati; nessuna dipendenza da acquisti per usare questo sprint.

Avvio: dalla root applicativa `./scripts/start-local-beta.ps1`; per ricompilare `./scripts/start-local-beta.ps1 -Build`. Stato finale: **7/7 servizi Docker sani**. Sito **https://localhost:3443**, Mailpit **http://localhost:8025**, monitor **http://localhost:9090**. Prossimo passo: collaudo manuale dei percorsi con tastiera e screen reader, poi definire l’incremento successivo con l’utente.
