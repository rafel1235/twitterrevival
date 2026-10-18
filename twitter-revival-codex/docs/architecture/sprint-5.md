# Sprint 5: sicurezza, moderazione e dati

## Confini e schema

Monolite Go/Chi, PostgreSQL e Next invariati. Migrazione 005: ruolo `member|moderator`, `posts.moderation_hidden_at`, `media.purged_at`, segnalazioni, audit e finestre di rate limit. Nessun Redis, servizio esterno o deploy. Le API verificano sempre sessione e ruolo attuali; il ruolo non viene letto da cookie/client. Registrazione e aggiornamento profilo non accettano ruoli.

Il rollback 005 elimina report/audit/quote e ruoli, rimuove l'oscuramento dei post e preserva account, sessioni, post e media. Va effettuato con API ferme, backup e binario Sprint 4: il binario Sprint 5 richiede schema 5. Non ricostruisce testi o oggetti già cancellati dalla manutenzione. I test verificano rollback/up su schemi separati.

## Segnalazioni e decisioni

`POST /reports`: sessione, Origin/CSRF, target post visibile o account attivo diverso dal segnalante, motivo enumerato e dettagli 10–2000 code point. Un report aperto per segnalante/target: invii ripetuti restituiscono il suo ID. Nessun automatismo di sospensione basato sul numero di report.

Il segnalante vede soltanto le proprie segnalazioni e motivazioni. Moderatori: coda filtrabile e paginata, testo del post anche se oscurato, stato account; nessuna email o credenziale degli utenti. Azioni motivate: archiviazione, oscuramento/ripristino post, sospensione/riattivazione account. Decisione, modifica dello stato e audit nello stesso commit. Due decisioni iniziali concorrenti: una riesce, l'altra riceve 409. Per annullare una restrizione si riapre la segnalazione gestita nella vista «Gestita». Gli account moderatori non sono sospendibili da altri moderatori; il gestore può prima revocare il ruolo tramite CLI.

La sospensione elimina sessioni e token di reset/verifica: la riattivazione richiede nuovo login. `post_audience` esclude contenuti oscurati; feed, FTS, anteprime, risposte, notifiche e download media riutilizzano quella vista. Ripristino dell'oscuramento distinto dal soft delete dell'autore. Non si ripristina un post eliminato.

La CLI `-role-user HANDLE -role moderator|member -reason MOTIVO` è l'unico bootstrap. Audit con operatore nullo identifica CLI locale; nessun account privilegiato predefinito. Audit append-only nelle API: il proprietario PostgreSQL conserva naturalmente poteri amministrativi; nessuna promessa di registro crittograficamente immutabile.

## Concorrenza, quote e spam

Le scritture su account/social/post/media acquisiscono le righe utente in ordine UUID prima dei lock coppia/post/media. Cancellazione e sospensione usano lo stesso protocollo. Questo impedisce che una scrittura già autenticata ricrei dati personali dopo la cancellazione. Il confronto dell'hash password sotto lock impedisce cancellazioni/export con una password superata da un reset concorrente.

Quote persistite PostgreSQL per minuto UTC: pubblicazione 30/account, upload/delete media 20/account, segnalazioni 10/account, export 3/account, cancellazione 5/account, altre scritture 120/account. Tentativi invalidi consumano quota; richieste senza Origin/CSRF valido non consumano quella dell'account. Registrazione/login/logout mantengono anche 30/minuto per peer nel DB. Restano i limiti di burst per peer degli sprint precedenti. Next condivide il peer dell'API: limite conservativo globale, senza fidarsi di X-Forwarded-For. Gli identificatori peer sono hash SHA-256, pseudonimi, eliminati dopo 10 minuti; non costituiscono anonimizzazione forte.

Antispam: stesso testo case-insensitive e stesso riferimento reply/quote rifiutato per 10 minuti, anche dopo cancellazione; verifica serializzata sotto lock dell'autore. Testi in conversazioni diverse restano consentiti. Nessun CAPTCHA, classificatore automatico o blocco automatico basato sui link. Quote DB resistono al riavvio e sono condivise fra istanze.

Errori inattesi nei log riportano solo classe `database|timeout|canceled|dependency`. Panic senza path/query/body. Nessun dump di errori pgx/S3, email, cookie, password o token. Gli access log applicativi con URL utente non sono abilitati.

## Esportazione e cancellazione

Entrambe richiedono password attuale, Origin/CSRF e sessione; password sbagliata 403 mantiene la sessione. Export ZIP privato e no-store: `data.json` con campi esplicitamente ammessi, immagini elaborate in `media/ID.png|jpg`, riferimenti a post/posizione. Account/profilo propri, post anche soft-deleted ancora conservati, following, blocchi, like, repost, notifiche ricevute, report inviati e restrizioni sui propri oggetti. Nessun hash, sessione/token, chiave S3 o email altrui. Limite automatico 100 MiB immagini, 32 MiB JSON, 10.000 righe per categoria; oltre limite risposta 413 e procedura assistita da definire con il gestore, senza troncamento silenzioso. Un export per istanza, file temporaneo con permessi privati eliminato al termine; timeout 60 secondi, nessun archivio persistito nel DB. Un arresto anomalo può lasciare un file temporaneo: vedere registro rischi.

Cancellazione con `confirmation: ELIMINA`: transazione che sostituisce handle/email/hash con tombstone inutilizzabili, azzera profilo, elimina il testo di tutti i post, revoca/elimina sessioni e token, rimuove relazioni/notifiche/interazioni e disassocia i dati del segnalante. UUID/tombstone dei post restano per preservare risposte altrui e FK; non sono dati anonimi garantiti. I report di terzi e motivazioni di audit possono contenere riferimenti personali: conservazione limitata e revisione del gestore, da validare legalmente.

## Retention e storage

Manutenzione nel processo API all'avvio e ogni minuto, batch di 200 oggetti. `deleted_at` con `purged_at` nullo è una coda durevole. Prima tombstone/visibilità DB, poi DELETE S3 idempotente e conferma nel DB; un guasto lascia il lavoro per il tentativo successivo. `-maintenance` permette esecuzione manuale e segnala pending con exit nonzero. Nessun retry finto in memoria.

- Bozze non associate: 24 ore, poi tombstone e purge.
- Singoli post eliminati: corpo sostituito e immagini eliminate dopo 30 giorni.
- Cancellazione account: testi subito, tutte le immagini in coda immediata.
- Sessioni scadute/token scaduti: pulizia; record revocati/consumati entro il giorno successivo.
- Report chiusi: 180 giorni dalla revisione; audit: 180 giorni. Report aperti: coda umana, nessuna cancellazione automatica che distrugga un caso ancora da esaminare.

`-reconcile-media` scorre le pagine S3 `images/` ed elimina solo oggetti senza record DB e con LastModified oltre 24 ore. È un comando operativo esplicito, non una scansione costosa ogni minuto; protegge upload in corso. Lo storage degli sprint precedenti rimane privato. Backup/restore: [procedura](../operations/backup-restore.md). Privacy e applicabilità DSA: [bozza da revisionare](../legal/privacy-dsa-review.md).
