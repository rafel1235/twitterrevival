# Sprint 8 — community tematiche nella beta locale

Incremento richiesto dall’utente dopo lo Sprint 7: gruppi pubblici con iscrizione libera e post riservati ai membri. La specifica iniziale escludeva le community dal primo MVP; questo documento registra l’estensione autorizzata. Dominio, server pubblico e distribuzione restano rimandati.

## Regole del prodotto

La directory `/communities` e le pagine `/communities/{id}` richiedono una sessione della beta. Nome, descrizione, regole, numero dei membri ed elenco dei membri sono consultabili senza iscriversi alla singola community. I blocchi reciproci con il proprietario, la sua sospensione/eliminazione e l’archiviazione rendono la community indisponibile. L’elenco dei membri omette account non attivi e persone bloccate reciprocamente dal lettore; il conteggio include i membri attivi.

Ogni account attivo può creare al massimo dieci community attive. La creazione iscrive automaticamente il proprietario, che può modificarne nome, descrizione e regole. Qualsiasi persona della beta può iscriversi liberamente e abbandonare una community; non sono previsti approvazione, inviti alla singola community o gruppi chiusi. Il proprietario non può uscire da una community attiva: riceve `409` e può archiviarla. Non sono previsti trasferimento della proprietà o ripristino dall’interfaccia.

I post, le risposte, le immagini e le interazioni richiedono un’iscrizione corrente, oltre ai controlli su account, blocchi e moderazione. L’uscita revoca l’accesso ai post, inclusi quelli pubblicati dall’account uscente, ma conserva la cronologia per gli altri membri. L’autore può ancora cancellare un proprio post tramite API dopo l’uscita e dopo l’archiviazione, purché gli account coinvolti siano attivi; la sospensione del proprietario può impedirne la cancellazione individuale. Rimane disponibile il percorso di eliminazione account.

Il feed della community è cronologico e paginato. I post della community non compaiono in home, profili o ricerca generale. Le risposte ereditano la community dal post originale nel backend; la destinazione non può essere cambiata dopo la creazione. Repost e citazioni dei post della community non sono consentiti in questo sprint, nemmeno all’interno della community.

## Architettura e persistenza

Lo stack resta Next.js/React/TypeScript con API Go/Chi e PostgreSQL. Non vengono aggiunti servizi o pacchetti. Il modulo `internal/communities` contiene servizio, query parametrizzate e validazione; gli handler HTTP usano autenticazione, origine esatta, protezione CSRF e limiti già presenti. Il browser continua a usare il proxy same-origin `/api/v1`.

La migrazione `007` aggiunge:

- `communities`: proprietario, nome, descrizione, regole, date e archiviazione. `owner_id` è nullable per l’eliminazione dell’account.
- `community_members`: coppia univoca community/utente e data d’iscrizione, con chiavi esterne e indici per utente e cursore.
- `community_events`: eventi `create`, `join`, `leave`, `update`, `archive`, senza testi dei post o dei metadati.
- `posts.community_id`, coerenza tra community e visibilità, indici e guardie SQL contro modifiche della destinazione, riferimenti incompatibili e repost.
- `community_audience`, usata per i metadati, e aggiornamento di `post_audience`/`post_cards` per applicare l’iscrizione anche ai percorsi esistenti.

Le mutazioni acquisiscono prima i lock degli account coinvolti in ordine UUID, poi quello della community; per i post seguono lock della coppia utenti, del post e dei media. Uscita, archiviazione e pubblicazione/interazioni non possono superare il controllo di accesso all’interno di una scrittura concorrente. Il lock del proprietario serializza anche il limite delle dieci community.

## Contratto HTTP

Tutti gli endpoint seguenti richiedono autenticazione. Le mutazioni richiedono anche origine/CSRF validi e un corpo JSON, incluso `{}` per le operazioni senza campi.

| Metodo e percorso                            | Risultato                                                                              |
| -------------------------------------------- | -------------------------------------------------------------------------------------- |
| `GET /api/v1/communities`                    | `200`, `data.items`, cursore opzionale; `mine=1` limita alle iscrizioni correnti       |
| `POST /api/v1/communities`                   | `201`, `data.community` e header `Location`; creazione con iscrizione del proprietario |
| `GET /api/v1/communities/{id}`               | `200`, `data.community`                                                                |
| `PATCH /api/v1/communities/{id}`             | `200`, `data.community`; aggiornamento parziale, solo proprietario                     |
| `DELETE /api/v1/communities/{id}`            | `204`; archiviazione del proprietario o di un moderatore di Radura                     |
| `GET /api/v1/communities/{id}/members`       | `200`, `data.items` con i soli campi pubblici dei profili e cursore opzionale          |
| `POST /api/v1/communities/{id}/membership`   | `204`; iscrizione idempotente                                                          |
| `DELETE /api/v1/communities/{id}/membership` | `204`; uscita idempotente, `409` per il proprietario                                   |
| `GET /api/v1/communities/{id}/posts`         | `200`, pagina di post; `403` se la community è visibile ma il lettore non è membro     |
| `POST /api/v1/posts`                         | Contratto esistente esteso con `community_id` per i nuovi post della community         |

`Community` espone `id`, `owner_id`, `name`, `description`, `rules`, `created_at`, `updated_at`, `members_count`, `is_member`, `is_owner`, `can_manage`. Quest’ultimo abilita l’archiviazione del moderatore, senza conferirgli accesso ordinario ai post o modifica dei metadati. La lista dei membri espone `id`, `handle`, `display_name`, `avatar_style`.

Nome: 3–60 code point Unicode; descrizione: 0–500; regole: 0–1000. Il server normalizza gli spazi esterni e rifiuta UTF-8 non valido e controlli, salvo newline/tab nei campi multilinea. Errori di campi e cursore restituiscono `422`; UUID malformati nelle risorse community restituiscono `404`, come risorse inesistenti o non visibili. Limite: 1–50, predefinito 20; cursore ordinato per data e UUID, senza offset.

## Moderazione e ciclo di vita

Le segnalazioni dei post riusano il flusso esistente. Per nome, descrizione o regole, l’interfaccia invita a segnalare il profilo del proprietario indicando la community. I moderatori di Radura possono leggere l’evidenza dei post segnalati nella coda di moderazione anche senza essere membri; questa eccezione non apre loro il feed ordinario. Possono archiviare una community visibile, con evento di audit. Non viene aggiunto un ruolo di moderatore della singola community.

L’esportazione personale include post con la relativa community, community possedute, proprie iscrizioni ed eventi riferiti all’account. L’eliminazione account archivia e anonimizza le community possedute, azzera `owner_id`, rimuove le iscrizioni pertinenti e scollega l’attore dagli eventi. La manutenzione applica agli eventi una retention di 180 giorni. L’archiviazione ordinaria non equivale a cancellazione fisica o rimozione immediata dai backup.

Il rollback `007` rifiuta atomicamente la rimozione dello schema se esiste qualunque post con `community_id`, inclusi i post cancellati logicamente: eliminare quella destinazione potrebbe allargarne la visibilità. Non viene convertito alcun post riservato in pubblico.

## Verifica

I test riguardano validazione Unicode, persistenza e cursori, autorizzazioni, blocchi, moderazione, immagini, uscita/archiviazione concorrenti, export/eliminazione e guardie di rollback. Le prove browser coprono creazione, iscrizione, lettura e pubblicazione con backend reale, gestione degli errori e layout accessibile. Gli esiti effettivi e i limiti della sessione sono registrati in `docs/implementation-status.md`; questo documento non anticipa il risultato delle suite ancora in esecuzione.
