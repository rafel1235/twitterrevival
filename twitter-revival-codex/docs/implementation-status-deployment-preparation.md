# Stato implementazione — preparazione dominio e server

Data: **18 ottobre 2026**. Prodotto **Radura**. L’utente ha richiesto dominio e server prima dello Sprint 7; non possiede ancora né dominio né VPS. Budget indicativo **10–20 €/mese**, beta privata a inviti. Dominio scelto: **raduraapp.it**.

**Stato: preparazione locale completata e verificata; acquisto, DNS e deploy remoto da eseguire.** Il sito locale dello Sprint 6 rimane separato. Nessun account esterno creato, servizio pagato, record DNS modificato, accesso SSH o invio email a terzi. Nessuna funzione Sprint 7 introdotta.

## Stato iniziale e conservazione

Letti AGENTS.md e PROJECT_SPEC.md. Repository inizialmente pulito su `main`, commit `e9d5684 sprint6`. Nessun commit/push/reset/cambio ramo; specifiche conservate. `.env`, segreti e volumi locali non modificati. Registro completo Sprint 6 archiviato in [implementation-status-sprint-6.md](implementation-status-sprint-6.md).

## Scelte e verifiche del dominio

- Nome visibile Radura, dominio scelto esplicitamente `raduraapp.it`.
- WHOIS ufficiale Registro .it: radura.it registrato (`ok`), raduraapp.it e radurasocial.it `AVAILABLE` al controllo. RDAP ufficiale Verisign: radura.com e radura.net registrati. Estratti soltanto nome/stato, nessun dato del titolare conservato.
- Proposta VPS: OVHcloud VPS-2, datacenter UE standard, 4 vCore/8 GB/75 GB, Ubuntu 24.04 LTS x86-64. Listino da 8,80 €/mese IVA inclusa; .it 3,65 € primo anno e 10,97 €/anno al rinnovo. Prezzi e disponibilità da riconfermare nel carrello, nessun ordine avviato.
- Dettagli, fonti ufficiali, opzioni di acquisto e procedura in [domain-server.md](operations/domain-server.md).

## File creati e attività completate

```text
compose.production.yaml
.env.production.example
.gitattributes
infra/Caddyfile.production
scripts/production-config.mjs
scripts/production-config.test.mjs
scripts/production-compose.test.mjs
scripts/start-production.sh
docs/operations/domain-server.md
docs/implementation-status-sprint-6.md
```

Aggiornati README, ignore Git/Docker, package scripts, workflow CI interno e nella root Git effettiva. Nessuna nuova dipendenza npm/Go, nessuna modifica al codice applicativo.

- Compose server autonomo, progetto/volumi `radura-production` separati; non eredita porte o Mailpit dallo sviluppo. Solo 80/443 pubbliche; monitor su loopback; DB/storage/API/frontend senza porte host.
- Caddy con ACME pubblico, volume certificati persistente, metriche/health interne nascoste, intestazioni e noindex per beta. `PUBLIC_BETA_ENABLED=false` restituisce vera manutenzione 503; health container 200.
- Modalità `start-production.sh --domain-only`: valida dominio/contatto e beta chiusa, avvia soltanto il gateway senza dipendenze. Consente dominio/HTTPS prima di SMTP e apertura applicativa. Non eseguita sul server, che non esiste ancora.
- Modalità completa: Origin HTTPS esatto, cookie Secure, inviti obbligatori, SMTP STARTTLS; credenziali email locali escluse, casella Mailpit assente dal build server.
- Init CSPRNG per segreti distinti, `.env.production` privata e mai sovrascritta. Parser senza eval/source, validazione placeholder/DSN/relay, secret monitor leggibile da Prometheus non root ma protetto dalla directory privata host.
- Limiti memoria, rotazione log, healthcheck; script con progetto esplicito e ambiente server autorevole. Nessun deploy automatico, migrazione dei contenuti locali o servizio a pagamento attivato.
- File shell con checkout LF; segreti `.env*` e cache TypeScript esclusi anche dal contesto Docker.

## Esiti effettivi

| Verifica eseguita                                       | Risultato                                                                                                                                                 |
| ------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `npm run test:deployment`                               | **9/9 passati**, nessuno saltato                                                                                                                          |
| Configurazione/credenziali su filesystem Linux effimero | **4/4 passati**, compresi permessi 600 e init senza sovrascrittura                                                                                        |
| Compose server                                          | Topologia e porte verificate tramite configurazione JSON, senza stampare segreti reali                                                                    |
| Avvio gateway con Compose                               | Riuscito in progetto `radura-domain-check-*` isolato, rete interna e nessuna porta host; unico container gateway sano; risorse proprie rimosse al termine |
| Caddy                                                   | Adapt entrambi gli stati senza rete/ACME; HTTP 503 manutenzione e 404 metriche verificati realmente in container offline                                  |
| Script server                                           | Sintassi Bash verificata in Linux; preflight dominio chiuso e config completa verificati                                                                  |
| Build Next per server                                   | **Passata**, immagine separata `radura-web:deployment-check`, `NEXT_PUBLIC_DEV_MAILBOX_URL` vuoto; nessuna sostituzione del container locale              |
| Format e diff                                           | Prettier e controllo whitespace passati, compresa la documentazione finale                                                                                |

I test temporanei usano soltanto valori sintetici e directory `.local` proprie, senza accedere ai dati applicativi esistenti. Nessun certificato pubblico richiesto: il test gateway usa HTTP su loopback nel container e non espone porte dell’host. L’avvio effettivo dello script remoto, il firewall VPS e HTTPS pubblico non sono ancora verificabili senza acquisto e accesso al server.

Non ripetute le suite Go e browser dello Sprint 6 perché il codice applicativo è invariato. Gli esiti storici (45 Go + 31 sottocasi, race Linux, 42 browser HTTP + 3 HTTPS, carico e restore) restano nel registro archiviato e non sono presentati come nuove esecuzioni. CI aggiornata, nessuna pipeline remota eseguita.

## Attività ancora necessarie

1. Acquistare dominio e VPS nel proprio account, dopo verifica del carrello e del costo mensile; fornire l’IP pubblico e rendere disponibile l’accesso SSH autorizzato senza inviare chiavi private in chat.
2. Configurare OS/Docker/SSH/firewall, trasferire soltanto il sorgente, impostare il record A e avviare la modalità solo dominio. Verificare il certificato reale senza eccezioni TLS.
3. Prima di aprire l’app ai piloti: relay/mittente e consegna email verificati, testi/informativa/contatti aggiornati per l’installazione remota, account moderatore, backup applicativo DB+S3 e restore Linux con copia fuori VPS. Il backup giornaliero della VM non sostituisce la prova applicativa.
4. Aprire la beta soltanto dopo le verifiche e l’autorizzazione del gestore. Rimangono i limiti documentati dello Sprint 6: quote peer condivise dietro Next, singolo server senza alta disponibilità e nessuna osservazione D7 reale ancora disponibile.

La configurazione locale continua a usare `scripts/start-local-beta.ps1`; i comandi server sono esplicitamente separati e documentati. Nessun acquisto o deploy è stato simulato o dichiarato completato.
