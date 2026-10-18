# Sprint 4 · Immagini, ricerca e notifiche

Estensione dello stesso monolite Go/Chi, PostgreSQL e Next.js. Unico nuovo servizio: **SeaweedFS 4.48**, storage S3 locale con volume `media_data`. SDK AWS Go v2 ufficiale, endpoint esplicito, credenziali statiche locali e path-style. Nessun account cloud, Redis, motore di ricerca separato o deploy.

## Immagini

Upload multipart con un `file` e una `alt_text`. Solo JPEG/PNG, massimo 5 MiB di input, 4096 px per lato e 8 milioni di pixel. Formato decodificato e MIME devono coincidere. Il server controlla le dimensioni prima dell'allocazione, decodifica e **ricodifica** tutti i pixel: scarta filename, metadati e byte aggiunti. Output massimo 8 MiB, JPEG qualità 85 o PNG. Rifiutati SVG, GIF, MIME incongruente, file vuoti e dati non decodificabili. Orientamento EXIF non interpretato; nessuna scansione antivirus o supporto implicito ad altri formati.

Alt text obbligatorio di 1–200 code point senza controlli. Due elaborazioni simultanee, 20 mutazioni media/minuto per peer, massimo 12 bozze e 100 MiB di immagini attive per account. Il lock dell'account serializza verifica quota e inserimento anche fra istanze.

Chiavi oggetto casuali a 256 bit e bucket privato. Nel DTO solo ID, URL same-origin, MIME, byte, dimensioni e descrizione: mai chiavi S3, credenziali o URL firmate pubbliche. `GET /media/{id}/content` ricontrolla sessione e visibilità a ogni richiesta, poi legge dal bucket. MIME canonico, `nosniff`, CSP restrittiva e `private,no-store`. Il bucket nega accessi anonimi, verificato nei test.

Bozza leggibile solo dal proprietario attivo. Dopo l'associazione segue `post_audience`: post non cancellato, account attivi e nessun blocco in entrambi i versi. Anche il proprietario perde l'accesso al contenuto quando il post è cancellato. In Compose soltanto S3 8333 è pubblicato sul loopback; console/filer/admin restano nella rete interna.

La UI offre upload, preview reale, recupero bozze dopo refresh e cancellazione. Un file selezionato ma non caricato impedisce di pubblicare; errori conservano testo/selezione. Fino a quattro immagini caricate per post, in ordine, con testo obbligatorio. Media immutabili dopo pubblicazione; PATCH modifica solo il testo. Le citazioni indicano gli allegati e rimandano al post originale.

L'associazione blocca le righe media in ordine UUID, verifica proprietario, scadenza, cancellazione e riuso, quindi inserisce `post_media` nella transazione del post. Vincoli unici impediscono riuso/posizioni duplicate: due creazioni concorrenti hanno un solo successo. Bozze associabili per 24 ore, eliminabili anche dopo scadenza. DELETE rifiuta immagini associate (409), nasconde quelle altrui (404) ed è ripetibile dal proprietario.

DB e S3 non sono una transazione unica: fallimento DB dopo PUT comporta un tentativo di cancellazione compensativa. La cancellazione di una bozza registra prima il tombstone DB, poi elimina l'oggetto; errori S3 consentono retry. Un crash può lasciare oggetti orfani. Riconciliazione/purge durevoli e retention restano allo Sprint 5. Il soft delete del post nasconde gli allegati ma conserva righe e byte, che continuano a contare nella quota. Non è cancellazione GDPR.

Ambiente: `S3_ENDPOINT`, `S3_REGION`, `S3_BUCKET`, `S3_ACCESS_KEY`, `S3_SECRET_KEY`. Segreto minimo 32 byte; endpoint senza credenziali/query/path, HTTPS obbligatorio fuori da loopback e hostname locale `storage`. Timeout, retry limitati, redirect disabilitati. Il bucket locale viene inizializzato all'avvio e il healthcheck include l'accesso S3. Nessuna configurazione cloud attivata.

Riferimenti: [release SeaweedFS 4.48](https://github.com/seaweedfs/seaweedfs/releases/tag/4.48), [mini alla stessa versione](https://github.com/seaweedfs/seaweedfs/blob/4.48/weed/command/mini.go), [endpoint SDK AWS Go](https://docs.aws.amazon.com/sdk-for-go/v2/developer-guide/configure-endpoints.html).

## Ricerca

`GET /search?q=…&type=posts|users&cursor=…&limit=…`, sessione obbligatoria. Query 2–100 code point senza controlli, default posts, limite 20/max 50, quota 120 letture/minuto per peer. SQL parametrizzato, nessuna query remota.

Post: `search_document` generato da `to_tsvector('simple',body)`, indice GIN parziale sui post non cancellati, `websearch_to_tsquery`. Token multilingua essenziali: niente stemming italiano o rimozione accenti. Supporto parole/frasi/sintassi websearch PostgreSQL; indice aggiornato automaticamente alle modifiche. Risultati di entità post, senza duplicati repost, con DTO/immagini/filtri della timeline.

Persone: prefisso handle o sottostringa nome, mai email. `%`, `_` e backslash trattati letteralmente. Account inattivi e blocchi esclusi. Entrambe le liste ordinate per data/UUID discendenti e paginate con cursore; nessun ranking per rilevanza/autocomplete. Form GET e tab Post/Persone, ricerca rapida desktop, empty/error/retry e caricamento successivo. Rimane il percorso per handle esatto.

Riferimento: [PostgreSQL text search](https://www.postgresql.org/docs/current/textsearch-controls.html). Query e ricerca per sottostringa non sottoposte a load test: misurare prima della beta.

## Notifiche

Follow, like, repost, risposta e citazione; nessuna notifica verso sé stessi e nessun backfill dell'attività precedente alla migrazione. Trigger PostgreSQL scrivono **nella stessa transazione** dell'azione; vincolo unico destinatario/attore/tipo/soggetto evita doppioni. Nessuna coda esterna o email sociale.

Salvati ID e orari, senza copie del testo. Letture filtrate per account attivi, blocchi, esistenza del follow e visibilità dei post sorgente/bersaglio. Unfollow/unlike/annullamento repost rimuovono la notifica. Cancellazione di risposta/citazione rimuove la propria notifica; cancellazione originale rimuove quelle collegate. Il blocco cancella le notifiche della coppia in entrambi i versi, senza ripristino allo sblocco. Restano i lock per coppia dello Sprint 3; anche i trigger verificano ammissibilità e account.

GET `/notifications`: `{items,next_cursor?,unread_count}`, limite 20/max 50 e ordine data/UUID discendenti. Pagina/conteggio nello stesso snapshot. DTO: ID, tipo, attore pubblico, post_id opzionale, created_at e read_at nullable. GET `/notifications/unread-count` serve il badge. POST `/{id}/read {}` marca solo una notifica visibile del destinatario, ripetibile; 404 per notifiche altrui. POST `/read {}` marca tutte quelle attualmente visibili nello snapshot della query, anche nelle pagine non aperte. Mutazioni con origin/CSRF, JSON e quota 60/minuto.

Badge e pagina aggiornati ogni 30 secondi nelle schede visibili e al ritorno in foreground. Badge con timer dopo risposta, abort e identità utente; pagina con abort e numero richiesta per scartare risposte obsolete. Polling e mutazioni rileggono la prima pagina: limite esplicito per chi ha caricato pagine precedenti. Errori pagina con retry. Nessun SSE, push, aggregazione o menzione automatica.

## Migrazione e verifiche

La 004 aggiunge media/post_media, FTS e notifiche/trigger/view preservando account, sessioni, grafo, post e interazioni. Down annulla solo la 004; non elimina oggetti S3 automaticamente. API ferme e backup per rollback; test solo su schemi isolati.

Backup iniziale `.local/backups/before-sprint4.dump`. Configurazione S3 aggiunta a `.env` con segreto casuale, senza sovrascrivere variabili esistenti. Test storage nel bucket separato `radura-tests`, con pulizia oggetti; test quote preparano metadati di confine artificiali, poi upload reali.

Test Go: decode/MIME/dimensioni/metadati, quota/scadenza, proprietà, bucket anonimo negato, riuso concorrente, FTS/cursori/edit/blocchi, notifiche atomiche/idempotenti/read e rollback. Browser: upload/recupero bozza, refresh, ricerca post/persone, cinque tipi di notifica, letture persistite, blocchi e tre viewport. Polling verificato accelerando solo l'orologio del browser, con HTTP/database reali. Verificati anche sessione, post e hash dei byte immagine dopo riavvio storage/API/web. Risultati nel registro.
