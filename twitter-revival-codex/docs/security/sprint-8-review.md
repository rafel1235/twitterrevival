# Review tecnica Sprint 8 — confini delle community

Perimetro: community scopribili nella beta autenticata, iscrizione libera e contenuti accessibili ai membri. Nessuna distribuzione pubblica, acquisto di dominio, nuovo provider, invito alla singola community o ruolo di amministratore di gruppo. Lo stack e le dipendenze rimangono quelli del repository.

## Matrice di accesso

| Attore/stato                                                                      | Metadati ed elenco membri                     | Post, immagini e interazioni                                                                          | Gestione                                                 |
| --------------------------------------------------------------------------------- | --------------------------------------------- | ----------------------------------------------------------------------------------------------------- | -------------------------------------------------------- |
| Senza sessione valida                                                             | `401`                                         | `401`                                                                                                 | `401`                                                    |
| Persona attiva, community visibile, non membro                                    | Consentiti, con filtri sui profili dei membri | Feed `403`, singolo post/media non disponibile; può iscriversi                                        | Nessuna modifica o archiviazione                         |
| Membro attivo                                                                     | Consentiti                                    | Consentiti secondo blocchi, stato, visibilità e moderazione esistenti                                 | Uscita; nessun privilegio sui post altrui                |
| Proprietario attivo                                                               | Consentiti, iscrizione automatica             | Stessi controlli dei membri                                                                           | Modifica/archiviazione; uscita da community attiva `409` |
| Moderatore di Radura non membro                                                   | Metadati di community visibili                | Nessun accesso ordinario aggiuntivo; eccezione per l’evidenza dei post segnalati nella coda esistente | Archiviazione con evento; nessuna modifica dei metadati  |
| Community archiviata, proprietario non attivo o blocco reciproco con proprietario | Risorsa non disponibile                       | Nessuna lettura o interazione ordinaria                                                               | Nessun ripristino esposto                                |

Il conteggio dei membri include gli account attivi; l’elenco dei profili applica inoltre i blocchi reciproci con il lettore. Essere membro non rende accessibile un post di autore sospeso, eliminato, bloccato, cancellato o nascosto dalla moderazione.

## Confini nel backend e nel database

`community_audience` controlla account del lettore, proprietario, blocchi e archiviazione. La nuova condizione in `post_audience` richiede l’iscrizione corrente quando `community_id` è valorizzato. Le letture dirette, i riferimenti alle risposte, le immagini e le notifiche ereditano il confine dei post esistente; non è sufficiente nascondere un pulsante nell’interfaccia. Home, profili e ricerca generale escludono esplicitamente i post delle community anche per i membri.

Le risposte ereditano la community dal post originale nel server. Vincoli e trigger mantengono coerenti `visibility`/`community_id`, rendono immutabile la destinazione e rifiutano risposte con destinazioni diverse. Le citazioni e i repost che coinvolgono post di community sono respinti, anche per un membro. Non viene aggiunta una condivisione tra community o verso il feed generale.

Le mutazioni acquisiscono tutti gli account necessari in ordine UUID, poi la community, quindi i lock della coppia utenti/post/media quando presenti. Iscrizione, uscita, archiviazione e scrittura del post sono transazionali. Una scrittura concorrente si completa prima della revoca oppure ne osserva l’esito; non viene autorizzata da una sola verifica nel browser. La creazione e il controllo delle dieci community possedute sono serializzati sul proprietario.

L’autore può cancellare un proprio post dopo aver lasciato una community, e anche dopo l’archiviazione, attraverso il percorso dedicato che non restituisce il contenuto del post. Quel percorso richiede account attivi: la sospensione del proprietario può impedire la cancellazione individuale. L’eliminazione del proprio account rimane il percorso generale di rimozione dei dati personali. Non esiste accesso di lettura residuale ai post dopo l’uscita.

Le API richiedono sessione reale; le mutazioni richiedono l’origine esatta, header CSRF, JSON e limiti esistenti. Query parametrizzate, lunghezze Unicode e divieto dei caratteri di controllo restano nel backend. I valori utente vengono renderizzati come testo, senza HTML arbitrario. Nessun token o contenuto privato viene aggiunto allo storage persistente del browser.

## Audit, moderazione e dati personali

Gli eventi registrano ID community, attore, azione e data, senza vecchi/nuovi testi. Iscrizioni/uscite duplicate e aggiornamenti senza variazioni non producono eventi aggiuntivi. La manutenzione esistente elimina gli eventi oltre 180 giorni. L’archiviazione da parte di un moderatore è registrata senza richiedere un nuovo tipo di segnalazione o pubblicare l’evidenza.

Un membro può segnalare un post con il flusso esistente. Per nome, descrizione o regole, la UI indica di segnalare il profilo del proprietario specificando la community. I moderatori del sito possono leggere l’evidenza dei post segnalati anche se non iscritti: è un’eccezione esplicita di gestione degli abusi, comunicata nella pagina, che non equivale ad accesso al feed. Le community con iscrizione libera non vanno presentate come gruppi confidenziali o approvati dal gestore.

L’esportazione include i post dell’account con `community_id`, le community possedute, le proprie iscrizioni e i propri eventi. Non esporta i post degli altri membri come conseguenza dell’iscrizione. L’eliminazione account rimuove le iscrizioni pertinenti, archivia e anonimizza i metadati delle community possedute, azzera `owner_id` e scollega l’attore dagli eventi; i post dell’account seguono la cancellazione personale esistente. I dati degli altri membri non diventano pubblici per effetto dell’eliminazione del proprietario. L’archiviazione non è una cancellazione immediata da storage e backup.

## Migrazione e rollback

La migrazione `007` è incrementale e non modifica la destinazione dei post pubblici esistenti. Chiavi esterne, coppia univoca delle iscrizioni e controlli di visibilità supportano i controlli applicativi. Il rollback controlla prima qualunque post con `community_id`, comprese le cancellazioni logiche, e in quel caso fallisce atomicamente. Non deve essere aggirato eliminando la colonna o rendendo pubblici i contenuti: ripristinare codice/schema richiede una strategia che preservi la destinazione dei dati.

## Verifiche e limiti

La copertura aggiunta comprende validazione Unicode, persistenza e paginazione, autenticazione/CSRF, proprietario/non membro/moderatore, blocchi e sospensioni, immagini, assenza dai feed generali, divieto di quote/repost, risposte coerenti, concorrenza con revoca/archiviazione, export/eliminazione e rollback protetto. Le prove backend sono eseguite nel progetto Docker effimero già predisposto, senza dati o volumi dell’applicazione corrente. I test browser usano account sintetici e backend reale; gli errori simulati sono circoscritti ai test.

Gli esiti misurati vengono riportati in `docs/implementation-status.md` dopo l’esecuzione. Questo documento descrive il contratto e i controlli, senza certificare assenza di vulnerabilità, conformità legale o accessibilità completa. Restano il collaudo umano e i limiti operativi già dichiarati per la beta locale; nessun deploy pubblico è incluso.
