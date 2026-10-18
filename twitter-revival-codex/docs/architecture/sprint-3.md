# Sprint 3 · Post e timeline

Il modulo `internal/posts` estende lo stesso monolite Go/Chi; query sqlc, PostgreSQL e frontend Next esistenti. Nessun Redis, worker sociale, storage media o nuovo servizio. Solo post testuali; tutte le letture richiedono una sessione, coerentemente con i profili dello Sprint 2.

## Testo e ciclo di vita

Post da 1 a 280 **code point Unicode** dopo trim degli spazi ai bordi. Go usa `utf8.RuneCountInString`, React `Array.from` e PostgreSQL `char_length`. Il trim client usa la proprietà Unicode `White_Space`, coerente con `strings.TrimSpace` di Go: include NEL, esclude BOM; non usa il diverso `String.trim` JavaScript. Emoji composte e accenti combinati possono occupare più code point: non si promette compatibilità con conteggi esterni. Sono ammessi newline e tab; gli altri controlli Unicode, compreso NUL, vengono rifiutati. Il client non usa `maxLength`, che conterebbe unità UTF-16. Il server resta autorevole.

Per rendere esplicita la parte update del CRUD, il proprietario può modificare **solo il testo**, senza cambiare autore, data di pubblicazione, risposta o citazione. L'interfaccia mostra “Modificato”; non c'è cronologia delle revisioni in questo incremento. Modificare non riporta il post in cima alla timeline. Un post può essere originale, risposta o citazione, mai risposta e citazione contemporaneamente. Il testo è sempre obbligatorio, anche nelle citazioni.

DELETE effettua un soft delete con `deleted_at`, è ripetibile per il proprietario e non permette ripristino. Il corpo rimane nel database e nei backup: non è cancellazione GDPR. La relativa strategia di retention resta nello sprint dedicato. Modificare/eliminare un post altrui visibile restituisce 403; un post assente, cancellato o non visibile restituisce 404 senza rivelarne la causa. UUID o testo non validi restituiscono 422.

## Visibilità e contenuti collegati

La sola visibilità implementata è `public`, vincolata nel database; non equivale ad accesso anonimo alle API. Le view ordinarie `post_audience` e `post_cards` centralizzano filtri e proiezioni: autore e visitatore devono essere attivi, il post non cancellato, nessun blocco in entrambi i versi. Ogni query applicativa vincola `viewer_id` all'utente ottenuto dalla sessione; queste view non sostituiscono l'autorizzazione dei handler e non usano SECURITY DEFINER.

Risposte e citazioni sono post indipendenti. Il collegamento resta dopo cancellazione o blocco dell'originale; il contenuto incorporato diventa `null` e la UI mostra un segnaposto senza nome o testo dell'originale. Non si nasconde automaticamente il testo scritto da un terzo autore visibile. Anche il link di contesto di una risposta segue questa regola. I riferimenti vengono caricati in una seconda query in batch, con gli stessi filtri; al massimo **un livello** di incorporamento, senza ricorsione o loop di richieste.

Un account bloccato/inattivo è escluso anche come autore di repost. Un repost non consente di aggirare un blocco verso l'autore originale; si nascondono inoltre repost fra attori che si sono bloccati. Like e repost sono idempotenti con PK `(user_id,post_id)`; i contatori escludono account inattivi e interazioni bloccate rispetto al visitatore o all'autore. I blocchi conservano le righe storiche di like/repost: vengono filtrate, possono riapparire dopo sblocco. I follow invece vengono eliminati dal comportamento dello Sprint 2. Nuove interazioni, risposte e citazioni a un post non visibile restituiscono 404.

Testo e nomi sono renderizzati da React come testo, senza `dangerouslySetInnerHTML`, auto-embedding di siti o download di URL. Nessun email/hash/token nei DTO pubblici.

## Feed e cursori

La home autenticata contiene post propri e degli account seguiti, comprese risposte/citazioni, più i repost effettuati da sé o dagli account seguiti. Il profilo include i post e i repost del profilo visibile. Non ci sono suggerimenti algoritmici. L'eventuale home anonima contiene soltanto tre esempi chiaramente etichettati demo e un composer disabilitato; non viene mescolata al feed reale.

Un evento originale usa ID e data del post. Un repost usa il proprio UUID persistente e la propria data; viene etichettato con chi lo ha ripubblicato. Se si seguono autore e reposter possono comparire due eventi dello stesso post: scelta esplicita, ciascuno con identità distinta. Ripetere un repost già attivo non cambia ID/data; annullarlo e ripubblicare crea un nuovo evento. Il testo rimane un'unica entità condivisa.

Ordine `created_at DESC,event_id DESC`, cursore Base64URL di timestamp UTC e UUID, limite default 20/max 50, lookahead di una riga. Gli elenchi risposte usano la stessa convenzione con l'ID del post. La pagina include `next_cursor` solo se esistono altri eventi. Nuovi eventi più recenti del cursore si leggono aggiornando il feed; il cursore non è una prova di autorizzazione e ogni pagina rivaluta visibilità e follow. Più pagine non formano uno snapshot persistente: blocchi, eliminazioni e unfollow intervenuti nel frattempo vengono rispettati.

Ogni lettura multipla usa una transazione read-only `REPEATABLE READ`: eventi, card e riferimenti appartengono allo stesso snapshot della singola richiesta. Le card sono caricate in batch, senza una query per post. Gli indici coprono autore e cursore dei post, risposte e autore/cursore dei repost. Non è stato eseguito un load test; il fan-out in lettura e le query dei contatori vanno misurati prima della beta.

## Scritture e sicurezza

Le scritture a un post esistente acquisiscono prima l'advisory lock della coppia attore/autore usato da follow/block, poi il lock della riga del post. La visibilità viene ricontrollata **dopo** l'acquisizione dei lock, nella stessa transazione delle modifiche. Questo serializza blocchi e interazioni e impedisce che una scrittura verificata prima del blocco passi dopo il suo commit. Anche la creazione di risposte/citazioni blocca e ricontrolla l'originale; una cancellazione concorrente non può lasciare un'interazione nuova autorizzata su un post già cancellato. Test concorrenti coprono questi casi.

Tutte le nuove mutazioni richiedono sessione, origin esatto/Fetch Metadata, header anti-CSRF e JSON con campi conosciuti. L'autore deriva dal cookie; nessun parametro cliente può impersonarlo. Limite richieste di 8 KiB e quota separata di 90 scritture/minuto per peer, con 429/Retry-After; rimangono i limiti di frequenza degli sprint precedenti e il loro limite noto dietro il proxy Next.

La UI mostra pending, empty/error/retry e conferma della cancellazione. Pubblica soltanto dopo validazione e risposta server; nessun salvataggio finto in memoria o localStorage. Il testo resta nel form se la richiesta fallisce. Dopo una mutazione ricarica dal server la prima pagina delle liste presenti e il dettaglio: non conserva le pagine aggiuntive già caricate. Risposte obsolete vengono ignorate e richieste interrotte allo smontaggio della lista. Nessun polling, push o sincronizzazione fra sessioni: usare “Aggiorna” o ricaricare la pagina per azioni compiute altrove.

## Migrazione e verifiche

`003_posts.up.sql` crea posts, likes, reposts, view e indici senza cambiare account, sessioni o grafo sociale. `down` elimina solo questi dati/view; una seconda esecuzione annulla la 002. Prima dell'upgrade locale è stato salvato `.local/backups/before-sprint3.dump`, escluso da Git. I test di rollback usano schemi isolati nel database di test, mai il database applicativo.

Test Go: validazione di 280/281 emoji, accenti combinati e trim NEL/BOM, CRUD/autorizzazioni, CSRF, vincoli SQL, idempotenza, contatori, cursori con timestamp uguali e inserimenti fra pagine, originali/repost fuori grafo, blocchi in entrambi i versi, riferimenti oscurati, sospensioni, concorrenza e rollback. Test browser con backend reale: ciclo completo nelle tre viewport, input Unicode e testo HTML innocuo, persistenza dopo refresh, paginazione aggiuntiva desktop e screenshot. Gli helper di test rispettano il `Retry-After` delle sole risposte 429 auth, senza disabilitare il limiter o riprovare errori funzionali. Esiti effettivi nel registro di implementazione.

Riferimenti per le scelte transazionali: [PostgreSQL isolation levels](https://www.postgresql.org/docs/current/transaction-iso.html) e [PostgreSQL explicit locking](https://www.postgresql.org/docs/current/explicit-locking.html).
