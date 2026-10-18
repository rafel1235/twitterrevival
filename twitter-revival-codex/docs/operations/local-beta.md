# Avvio e verifica della beta locale

Tutti i comandi dalla directory `twitter-revival-codex`. Docker Desktop attivo; `.env` esistente da conservare.

```powershell
./scripts/start-local-beta.ps1 -Build
# Riavvio successivo senza ricompilare:
./scripts/start-local-beta.ps1
```

Equivalente Compose: prima `./scripts/prepare-sprint6.ps1`, poi `docker compose -f compose.yaml -f infra/https.compose.yaml --profile monitoring up -d --build --wait`. Lo script cerca Docker nel PATH o nell’installazione Desktop standard, senza modificare impostazioni globali. Scrive `.local/metrics-token` per il secret Prometheus e aggiunge il token a `.env` soltanto se assente. Non mostra segreti DB/S3/monitor.

## Certificato locale e browser

Aprire **https://localhost:3443**, con `localhost` esatto. Caddy genera la propria CA interna nel volume `caddy_data`: esportiamo soltanto `/data/caddy/pki/authorities/local/root.crt` in `.local/caddy-root.crt`. Il certificato pubblico non è una chiave privata. I volumi Caddy contengono invece materiale privato e non vanno condivisi.

Per evitare l’avviso TLS, controllare il certificato con `certutil -dump .local/caddy-root.crt`, quindi aprirlo da Windows e scegliere manualmente **Installa certificato → Utente corrente → Autorità di certificazione radice attendibili** soltanto per questa CA generata dalla propria installazione. Riavviare il browser. Non importare certificati da fonti sconosciute. Il progetto non modifica automaticamente l’archivio di fiducia. Eliminando il volume Caddy si genera una CA diversa: l’eventuale vecchia fiducia va rimossa/aggiornata manualmente.

Redirect locale: `http://localhost:3080` → `https://localhost:3443`. Il sito HTTP 3000 non è l’origine consentita alle mutazioni mentre è attivo l’overlay. Per tornare allo sviluppo HTTP: `docker compose -f compose.yaml -f infra/https.compose.yaml stop gateway`, poi `docker compose --profile monitoring up -d --wait`. Non eliminare volumi. Per HTTPS rilanciare lo script.

## Inviti e primi piloti

1. Per il bootstrap: `docker compose exec api /api -invite-email EMAIL`, conservare il token mostrato una sola volta e registrare l’account con quell’email. Account già presenti possono effettuare login senza invito.
2. Assegnare a un handle scelto dal gestore il ruolo: `docker compose exec api /api -role-user HANDLE -role moderator -reason "Gestione autorizzata della beta locale."`. Ricaricare la sessione.
3. Aprire `/moderation/beta`: invito personale con scadenza, revoca, capienza 30–50 e pausa ingressi. Condividere privatamente il link mostrato; nessun invio automatico è configurato.
4. L’installazione su loopback è raggiungibile soltanto dalla propria macchina. Per il collaudo usare browser/profili separati sullo stesso computer. Non promettere accesso remoto ai piloti in questa configurazione.
5. Quando si svolgerà un pilotaggio reale autorizzato: email verificata, primo follow, primo post e prima interazione; raccogliere feedback e controllare report/operazioni ogni giorno. Attendere almeno 8 giorni per il primo rapporto D7. Nessun risultato reale è stato generato artificialmente.

Le email di verifica/reset sono catturate da **http://localhost:8025**. Mailpit conserva al massimo 1000 messaggi e non è un servizio di consegna ai piloti remoti. È accessibile soltanto su loopback e va trattato come casella privata, perché contiene link di accesso temporanei.

## Monitor e interventi

Aprire **http://localhost:9090**: pagina Targets con `radura` UP e pagina Alerts. Query utili:

```promql
up{job="radura"}
sum(rate(radura_http_requests_total{status=~"5.."}[5m]))
histogram_quantile(0.95, sum by (le) (rate(radura_http_request_duration_seconds_bucket[5m])))
radura_mail_jobs_failed
radura_media_purge_pending
```

Cinque regole: API assente 1m, errori >5% per 2m, p95 >1,5s per 2m, email esaurite 1m, purge in attesa 10m. Le ultime due segnalano backlog; non identificano una persona/oggetto. Non c’è Alertmanager né recapito esterno: gli allarmi sono consultabili localmente. Le soglie sono iniziali e richiedono taratura sui piloti reali.

- API DOWN: `docker compose ps` e `docker compose logs --tail 100 api`; verificare schema, DB e S3.
- Errori/latency: distinguere 429 (quote), 499 (client disconnessi), 5xx. Non disattivare le quote per far passare un test. I browser condividono il peer Next, quindi possono raggiungere una quota conservativa comune.
- Email fallite: controllare Mailpit e log sanitizzati; il membro può richiedere nuovamente il link, riattivando un job esaurito. Non estrarre token dal DB.
- Purge: verificare S3; il retry automatico continua ogni minuto. I comandi di manutenzione/riconciliazione già esistenti possono eliminare dati scaduti: rispettare la procedura documentata.
- Incidente: mettere in pausa gli ingressi dal pannello. Per fermare tutto mantenendo i dati: `docker compose -f compose.yaml -f infra/https.compose.yaml --profile monitoring stop`.

Validazione riproducibile:

```powershell
docker compose --profile monitoring exec -T prometheus promtool check config /etc/prometheus/prometheus.yaml
docker run --rm --entrypoint promtool -v "${PWD}/infra/monitoring:/checks:ro" -w /checks prom/prometheus:v3.15.0 test rules alerts.test.yaml
docker compose -f compose.yaml -f infra/https.compose.yaml exec -T gateway caddy validate --config /etc/caddy/Caddyfile
```

## Test browser, TLS e carico

Suite completa HTTP, su stack configurato HTTP, prima del passaggio HTTPS:

```powershell
$env:E2E_AUTH='1'
$env:E2E_MAILPIT='1'
$env:PLAYWRIGHT_BASE_URL='http://localhost:3000'
npm run test:smoke
npm run test:load
```

Gli helper emettono inviti nella coorte `e2e-*`. Il carico accetta soltanto HTTP loopback, 30–50 utenti (default 50), 15–120 secondi (default 30); `LOAD_USERS` e `LOAD_SECONDS` regolano la prova. Crea soltanto account sintetici `load-*`, 10 post e follow, poi 50 utenti a una richiesta/secondo: 60% feed, 28% dettaglio, 8% profilo, 4% like. Setup e cleanup sono fuori dai percentili. Rispetta Retry-After; non eseguirlo insieme agli E2E o al backup. Elimina tramite API/password soltanto gli account creati da quella esecuzione; i tombstone restano secondo la politica esistente.

Risultato aggregato in `docs/testing/load-sprint-6.json`: p50/p95/p99, throughput, status, errori e cleanup. Nessuna password/sessione/email/token salvata nel rapporto. Soglie: almeno 100 richieste, p95 <1500ms, p99 <3000ms, nessun 5xx/errore rete/status inatteso, 429 ≤10%, cleanup completo. È un carico limitato, non una prova di saturazione o una stima per Internet.

Dopo l’avvio HTTPS e l’esportazione della CA:

```powershell
$env:PLAYWRIGHT_BASE_URL='https://localhost:3443'
npm run test:https
```

Tre viewport. Node verifica realmente catena CA e hostname con `rejectUnauthorized: true`. Chromium ignora l’errore della CA privata soltanto in una configurazione che accetta l’URL fisso locale; verifica cookie Secure/HttpOnly/SameSite/Path, persistenza, pubblicazione, Origin estraneo rifiutato, cancellazione e sessione revocata. Il test non installa CA e non disabilita la verifica TLS globale.

In API nativa gli helper usano `E2E_API_CLI` al binario e `DATABASE_URL` locale; in Compose usano `docker compose exec api /api`. Non stampare l’output CLI degli inviti nei log condivisi. Test backend e rollback usano schemi dedicati `radura_test` e bucket `radura-tests`, come descritto nel README.

## Backup e riferimenti

La [prova backup/restore](backup-restore.md) enumera automaticamente tutte le tabelle dello schema corrente (007, incluse community, iscrizioni ed eventi) e il volume S3. Non include `.env`, metriche Prometheus né CA privata Caddy: custodire separatamente i segreti necessari al ripristino. Una nuova CA può essere generata dopo un recupero, aggiornando la fiducia del browser. Nessun restore sui dati attivi. L’esecuzione della prova di restore non è stata ripetuta nello Sprint 8: i risultati precedenti e i limiti sono conservati nei relativi resoconti.

Riferimenti primari: [HTTPS automatico e CA locale Caddy](https://caddyserver.com/docs/automatic-https), [opzioni globali Caddy](https://caddyserver.com/docs/caddyfile/options), [configurazione Prometheus](https://prometheus.io/docs/prometheus/latest/configuration/configuration/), [regole di allarme](https://prometheus.io/docs/prometheus/latest/configuration/alerting_rules/). Versioni fissate: [Caddy 2.11.7](https://github.com/caddyserver/caddy/releases/tag/v2.11.7), [Prometheus 3.15.0](https://github.com/prometheus/prometheus/releases/tag/v3.15.0).
