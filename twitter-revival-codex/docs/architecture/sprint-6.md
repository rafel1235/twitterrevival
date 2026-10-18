# Sprint 6 — beta locale a inviti

Incremento autorizzato: beta e HTTPS esclusivamente locali. Il monolite Go/Chi, PostgreSQL, S3 locale e Next rimangono l’architettura applicativa. Caddy termina TLS e Prometheus raccoglie metriche; nessun cloud, messaggio esterno, DM o federazione.

## Ingressi e autorizzazioni

La migrazione 006 aggiunge `beta_cohorts`, `beta_invites`, `beta_invite_events`, `beta_members`, trigger per i primi eventi e `mail_jobs`. La coorte `pilot` parte con capienza 50; il moderatore può scegliere 30–50 e sospendere gli ingressi. Membri più inviti attivi occupano i posti, anche prima della registrazione.

Invito: 32 byte CSPRNG, Base64URL, solo SHA-256 in DB, email case-insensitive, scadenza 1–14 giorni, default 7. Un invito attivo per email/coorte; nessun invito per account già presente. Il codice compare solo alla creazione. La lista contiene gli ultimi 100 inviti piloti e non restituisce codici o hash. Emissione, consumo e revoca sono registrati; la revoca è idempotente. Il pannello non invia email agli invitati.

La registrazione valida l’invito prima di Argon2 e ripete il controllo in transazione. Ordine dei lock: coorte, invito, inserimento del nuovo utente; utente/profilo/sessione, consumo e membership sono atomici. Un conflitto handle/email non consuma il codice. Emissione e revoca si serializzano sulla stessa coorte: test concorrenti coprono consumo e ultimo posto. Login degli account precedenti preservato.

Le rotte di amministrazione verificano ruolo e stato nel DB, anche se la UI nasconde i controlli. Mutazioni con sessione, Origin esatto, Fetch Metadata e header CSRF; quote persistenti esistenti. Il bootstrap del moderatore è una CLI locale esplicita, senza credenziali predefinite. In `localhost` soltanto la CLI può creare coorti `e2e-*`/`load-*`; l’API pubblica di invito usa sempre `pilot`. Origini non locali rifiutano la disattivazione dell’obbligo invito e non ammettono coorti di test.

## Indicatori misurati

| Indicatore        | Definizione implementata                                                              |
| ----------------- | ------------------------------------------------------------------------------------- |
| Membri            | Registrazioni ammesse con invito `pilot`, escluse cancellazioni                       |
| Verificati        | Membri con email verificata                                                           |
| Primo follow      | Prima relazione follow inserita con successo                                          |
| Primo post        | Primo post inserito, comprese risposte/citazioni                                      |
| Prima interazione | Primo like, repost, risposta o citazione                                              |
| Attivati          | Email verificata, primo follow e primo post                                           |
| Ritorno D7        | Richiesta autenticata ricevuta fra 7 giorni inclusi e 8 giorni esclusi dall’ingresso  |
| Retention D7      | Ritorni D7 / membri entrati da almeno 8 giorni, dopo la chiusura dell’intera finestra |

Il denominatore D7 non include membri ancora nella finestra: `d7_rate: null` e UI «Non ancora misurabile» finché nessuno è idoneo. Aggiornamento dell’attività al massimo una volta al minuto, server side. Il polling autenticato può contare come attività: è un indicatore tecnico di ritorno, da interpretare con feedback dei piloti, non una prova di attenzione umana. I primi eventi persistono anche dopo un unfollow o la rimozione di un post. Non viene ricostruito il passato degli account preesistenti. Coorti sintetiche sempre escluse dalle statistiche pilota, nessun tracker di terzi.

I propri indicatori sono inclusi nel JSON dell’export. Cancellazione account: rimozione membership, inviti destinati alla propria email/usati dall’account, job email; disassociazione dell’attore/issuer negli eventi rimanenti. Inviti scaduti o revocati eliminati dopo 30 giorni, intenti email dopo 24 ore. I backup seguono la politica separata esistente.

## Remediation e osservabilità

La precedente coda email in memoria è sostituita da intenti PostgreSQL limitati a 32 job pendenti. Worker con claim `SKIP LOCKED`, lease 30s, timeout invio 15s e 5 tentativi totali; backoff 1, 2, 4, 8 minuti. L’ack controlla il numero di tentativo: un worker che ha perso la lease non cancella il lavoro ripreso da un altro. Il quinto fallimento resta visibile e una nuova richiesta lo riattiva. Ogni tentativo genera un token nuovo, memorizzato solo hashato in `auth_tokens`; nessun token riutilizzabile nei job. Consegna almeno una volta: un crash dopo SMTP e prima dell’ack può generare un secondo link; solo quello più recente è valido. Retry e cooldown non perdono l’intento dopo un riavvio.

`/metrics` richiede bearer dedicato di almeno 32 caratteri, non è inoltrato dal frontend ed è bloccato dal gateway. Contatori HTTP, istogrammi latenza, uptime, goroutine, connessioni DB e code email/purge. Label con metodo limitato, pattern Chi e status: mai URL grezzi, query, email, IP, contenuti o token. Il numero di serie è limitato. I contatori HTTP ripartono al riavvio; Prometheus conserva la storia 7 giorni.

Durante gli E2E il monitor ha rilevato richieste DB annullate dalla navigazione classificate 500: ora `context.Canceled` produce 499 tecnico, le scadenze 504. I veri 5xx e i panic recuperati restano conteggiati. Test dedicati distinguono i casi e verificano l’assenza di dati privati nelle metriche. Le quote auth su verifica/reset sono condivise anche in PostgreSQL.

## HTTPS, rollback e confini

Caddy con CA interna per `https://localhost:3443`, senza ACME pubblico e senza installazione automatica della CA in Windows. Cookie `__Host-radura_session`, Secure, HttpOnly, SameSite=Lax, Path=/, senza Domain. Origin HTTPS esatto fino all’API tramite Next. SMTP senza STARTTLS ammesso esclusivamente nell’overlay locale con origine loopback ed endpoint `mailpit:1025`; resta obbligatorio negli altri casi HTTPS.

Rollback 006 testato su schema isolato: elimina soltanto tabelle/trigger beta e job, preserva utenti/sessioni/post/media. Richiede stop API e binario Sprint 5; si perdono indicatori/inviti/intent mail, da esportare o gestire prima. Nessun rollback sui dati attivi è stato eseguito.

Non sono stati invitati 30–50 esseri umani e non esistono risultati reali D7. Il carico sintetico verifica un profilo riproducibile della macchina locale; non stabilisce capacità di produzione. Un eventuale lancio esterno richiederà un incarico distinto e la chiusura delle questioni operative e giuridiche già documentate.
