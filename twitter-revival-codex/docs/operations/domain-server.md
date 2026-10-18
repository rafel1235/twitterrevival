# Radura: dominio e primo server

Preparazione del 18 ottobre 2026, precedente allo Sprint 7. Scelte dell’utente: marchio **Radura**, dominio desiderato **raduraapp.it**, beta privata a inviti, budget indicativo **10–20 €/mese**. Nessun account esterno, dominio o VPS acquistato; nessuna modifica DNS, accesso SSH o pubblicazione eseguiti.

## Acquisto proposto, da confermare nel carrello

| Voce                 | Configurazione proposta                                                                                           | Prezzo pubblicizzato al controllo                               |
| -------------------- | ----------------------------------------------------------------------------------------------------------------- | --------------------------------------------------------------- |
| VPS OVHcloud VPS-2   | Datacenter UE standard, 4 vCore, 8 GB RAM, 75 GB NVMe, IPv4 e backup giornaliero inclusi; Ubuntu 24.04 LTS x86-64 | Da **8,80 €/mese IVA inclusa**                                  |
| Dominio raduraapp.it | Solo dominio e zona DNS, nessun hosting web aggiuntivo                                                            | .it **3,65 € IVA inclusa primo anno**, rinnovo **10,97 €/anno** |

Fonti: [VPS e opzioni OVHcloud](https://www.ovhcloud.com/it/vps/), [listino .it e rinnovo](https://www.ovhcloud.com/it/domains/tld/it/). Sono prezzi “da” e promozionali: verificare ciclo di fatturazione, durata dell’impegno, regione e totale finale prima del pagamento. Preferire un piano mensile; non acquistare Plesk, cPanel, Windows, hosting WordPress o componenti non richiesti. Scegliere un datacenter standard in Francia/Germania se disponibile: la documentazione del provider indica limitazioni Docker nelle Local Zone. Il budget lascia un margine per copie esterne o email, da scegliere prima di attivare ulteriori costi.

Controllo registri, 18 ottobre: WHOIS ufficiale `whois.nic.it:43` → `radura.it` stato `ok`, `raduraapp.it` e `radurasocial.it` stato `AVAILABLE`; RDAP Verisign → `radura.com` e `radura.net` già registrati. Sono stati estratti soltanto dominio/stato, senza conservare dati dei titolari. La disponibilità non è una prenotazione né una verifica dei diritti sul nome: riconfermarla nel carrello. [Ricerca ufficiale Registro .it](https://web-whois.nic.it/).

La registrazione e il pagamento richiedono il proprio account presso il provider. Dopo l’acquisto servono l’IP pubblico assegnato e l’accesso SSH autorizzato; password, chiavi private e dati di pagamento restano fuori dalla chat e dal repository.

## Configurazione preparata

`compose.production.yaml` è **autonomo**: non unirlo al Compose di sviluppo. Il progetto `radura-production` usa volumi distinti e non migra automaticamente gli account o i contenuti sintetici del computer locale.

- Caddy espone soltanto TCP 80/443 e usa ACME per un certificato pubblico del dominio, con rinnovo e volume persistente. Niente CA locale sul server.
- PostgreSQL, storage SeaweedFS, migrator, API e Next non pubblicano porte host. Rete backend interna; API collegata anche alla rete con uscita per il relay SMTP.
- Prometheus su `127.0.0.1:9090`, consultabile tramite tunnel SSH; `/metrics` bloccato dal gateway, bearer dedicato interno. Nessuna notifica esterna configurata.
- HTTPS obbligatorio per cookie `__Host-`, Origin `https://DOMAIN`, beta sempre a inviti. Le coorti di test locali non sono ammesse con origine pubblica.
- SMTP reale con STARTTLS obbligatorio su 587/2525; niente Mailpit o credenziali locali. La porta 465 con TLS implicito non è supportata dal mailer attuale.
- Casella di sviluppo esclusa dal build Next; limiti memoria e rotazione dei log. Nessuna automazione di deploy/pull/reboot non richiesta.
- `PUBLIC_BETA_ENABLED=false`: il dominio mostra **503 «Radura: beta privata in preparazione»**. Il certificato può essere emesso anche con la beta chiusa. Il controllo container interno rimane 200.

I 503 di manutenzione sono intenzionali. Verificare il certificato con il browser o `curl` senza `-k`; per il collaudo applicativo attendere la preparazione effettiva della beta remota. La pagina privacy e alcuni testi dell’app descrivono ancora l’anteprima locale: non abilitarne l’accesso remoto senza aggiornarli con dati e contatti reali del gestore e completare la revisione già prevista.

## Procedura dopo l’acquisto e l’accesso al VPS

1. Installare Ubuntu 24.04 LTS, aggiornamenti e Docker Engine con plugin Compose dal [repository ufficiale Docker](https://docs.docker.com/engine/install/ubuntu/). Configurare SSH con chiave personale e verificare una seconda sessione prima di cambiare l’accesso amministrativo.
2. Aprire TCP 80/443; proteggere SSH e limitare l’accesso amministrativo ove possibile al proprio IP. Le porte Docker pubblicate possono aggirare UFW: la configurazione evita di pubblicare DB/storage/API, non si affida soltanto al firewall host. [Spiegazione ufficiale Docker](https://docs.docker.com/engine/network/packet-filtering-firewalls/#docker-and-ufw).
3. Trasferire il sorgente in una cartella posseduta dall’operatore, per esempio `/opt/radura`. Escludere `.env*`, `.local`, `.git`, `node_modules`, `.next`, report e backup; includere invece `.env.production.example`. Non caricare database/immagini/account sintetici automaticamente. Conservare la copia locale funzionante.
4. Configurare la zona DNS del dominio, non soltanto un inoltro URL:

| Tipo | Nome | Valore                                                       |
| ---- | ---- | ------------------------------------------------------------ |
| A    | `@`  | IPv4 pubblico del VPS                                        |
| AAAA | `@`  | Solo se IPv6 del VPS è realmente configurato e raggiungibile |

Non inserire `localhost`, 127.0.0.1 o l’IP privato del PC. Non aggiungere `www` a questa prima configurazione: il nome canonico è `raduraapp.it`. Eventuali record AAAA/CAA preesistenti vanno verificati per non impedire il certificato. Nessun record DNS è stato modificato durante la preparazione.

5. Sul VPS generare segreti nuovi (non quelli di sviluppo). Con Node 22+:

```sh
cd /opt/radura
node scripts/production-config.mjs init raduraapp.it EMAIL_REALE_OPERATORE
```

Senza Node host, usare il runtime già impiegato dal frontend:

```sh
docker run --rm --network none --user "$(id -u):$(id -g)" \
  -v "$PWD:/workspace" -w /workspace node:24-alpine \
  node scripts/production-config.mjs init raduraapp.it EMAIL_REALE_OPERATORE
```

La prima esecuzione Docker può richiedere `docker pull node:24-alpine`. L’operatore deve avere accesso a Docker; non cambiare i permessi del socket per renderlo pubblico. Il gruppo Docker dà privilegi amministrativi sul server.

Il comando crea `.env.production` con permessi 600 e segreti indipendenti DB/S3/monitor; se il file esiste **si ferma senza sovrascriverlo**. Non ruotare POSTGRES_PASSWORD su un volume già inizializzato. Il parser non esegue il file ambiente: niente `source`, `eval` o interpolazioni. Password SMTP contenenti `$` o `#` richiedono apici singoli; evitare apici interni.

**Per completare prima il solo dominio/server**, senza attendere il relay email:

```sh
# Dopo acquisto, DNS corretti e accesso al VPS autorizzato:
bash scripts/start-production.sh --domain-only
```

Questa modalità controlla dominio, contatto e stato chiuso, poi avvia soltanto Caddy con `--no-deps`. Non avvia database, API o frontend e non richiede ancora credenziali SMTP. Il dominio mostra la pagina di manutenzione con certificato pubblico; questa è la prima fase richiesta dall’utente. L’avvio completo sotto è una fase successiva, da completare dopo le impostazioni applicative mancanti.

6. Configurare un relay SMTP e un mittente verificato `Radura <no-reply@raduraapp.it>` nei campi SMTP. Inserire i record di verifica, DKIM/SPF/DMARC forniti dal servizio effettivo: non inventare valori. Verificare STARTTLS e la consegna con un proprio indirizzo di test prima di invitare persone. Un possibile servizio da valutare è [Brevo](https://help.brevo.com/hc/en-us/articles/208580669-FAQs-What-are-the-limits-of-the-Free-plan), che pubblicizza 300 email/giorno nel piano gratuito; nessun account o invio è stato creato. La compatibilità TLS/consegna del relay scelto non è stata ancora provata dal VPS.

7. Validare e avviare **dopo l’approvazione del deploy sul server scelto**:

```sh
node scripts/production-config.mjs check
bash scripts/start-production.sh
```

Il comando Bash usa Docker per il preflight anche senza Node installato sull’host. Rende autorevole `.env.production`, ignorando eventuali omonime variabili di sviluppo esportate. Prepara il solo secret monitor in `.local/production/metrics-token`: directory 700, file 644 per il bind leggibile dal processo Prometheus non root; la directory impedisce la lettura agli altri utenti host. File ambiente e token non vanno pubblicati o allegati ai log.

Avvio applicativo e migrazioni sono reali, ma non sono stati eseguiti su un VPS inesistente. Lanciare questo comando dopo la risoluzione DNS: Caddy contatta un’autorità ACME per emettere il certificato. Non ripeterlo per aggiornare un’istanza con dati senza un backup verificato e una procedura di aggiornamento.

8. Verificare dal proprio PC:

```sh
curl -I https://raduraapp.it
# In manutenzione: 503, certificato attendibile senza eccezioni.
ssh -L 9090:127.0.0.1:9090 OPERATORE@IP_VPS
```

Con il tunnel aperto, il monitor è raggiungibile su `http://localhost:9090`. Se il Prometheus locale occupa già quella porta, usare `-L 19090:127.0.0.1:9090` e aprire la porta locale 19090. Non aprire 9090 nel firewall pubblico.

## Prima dell’apertura ai piloti

Preparare informativa/contatti e testi coerenti con l’installazione remota, relay verificato, account moderatore e inviti personali. Definire backup applicativo DB **e** S3 con copia fuori VPS e prova di restore: il backup giornaliero della VM non sostituisce quella verifica. La procedura PowerShell esistente riguarda l’ambiente locale; un backup/restore operativo Linux remoto resta da completare sul server scelto. Il presente incremento non attiva nuovi servizi di backup o scheduler.

La modifica `PUBLIC_BETA_ENABLED=true` apre il proxy all’app: non effettuarla durante la sola configurazione dominio/server. Le quote per peer rimangono condivise dietro Next, limite conservativo già documentato; controllare errori/429, memoria e spazio (`docker stats`, `df -h`) prima di ampliare la beta. Un VPS singolo non fornisce alta disponibilità.

## Verifiche locali della preparazione

```sh
npm run test:deployment
npm run format:check
```

Nove test: topologia/porte e ambiente, Caddy nei due stati senza rete/ACME, avvio reale del solo gateway con Compose in un progetto temporaneo isolato senza porte host, gateway HTTP offline con 503 e 404 reali, sintassi Bash, validazione segreti/relay, rifiuto interpolazioni, init senza sovrascrittura e preparazione del token. La modalità solo dominio accetta SMTP ancora da configurare ma rifiuta l’apertura della beta. Quattro test di configurazione eseguiti anche su filesystem Linux effimero, compreso il rifiuto dei permessi ambiente non privati. Il test HTTP non è una prova di certificato pubblico né dell’intera app remota. La build server del frontend è separata dall’immagine locale e non avvia container del sito.
