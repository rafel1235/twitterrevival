# Bozza privacy e DSA per revisione del gestore e consulenza legale

Versione tecnica del 18 ottobre 2026. Descrive un prototipo locale. **Non è un'informativa pubblicabile, una valutazione legale o una dichiarazione di conformità.** Non attribuisce obblighi o esenzioni al futuro gestore senza determinarne caratteristiche e giurisdizione.

## Riferimenti primari consultati

Il [GDPR, regolamento UE 2016/679](https://eur-lex.europa.eu/legal-content/IT/TXT/?uri=CELEX:32016R0679) disciplina, fra l'altro, minimizzazione/conservazione, informazioni agli interessati, cancellazione e portabilità (artt. 5, 12–13, 17, 20). La disponibilità di un pulsante export/delete non dimostra da sola il rispetto del regolamento; basi giuridiche, diritti applicabili ed eccezioni vanno valutati sul trattamento concreto.

Il [DSA, regolamento UE 2022/2065](https://eur-lex.europa.eu/legal-content/EN/ALL/?uri=CELEX:32022R2065) prevede meccanismi di notice-and-action e motivazioni delle restrizioni (artt. 16–17); contiene anche disposizioni sui reclami e specifiche regole per micro/piccole imprese (artt. 19–20). L'applicabilità delle varie sezioni e delle esenzioni va verificata, senza assumere che la piccola dimensione esenti dall'intero regolamento. I report autenticati del prototipo non sostituiscono automaticamente il canale previsto per notice esterne.

## Campi da completare prima dell'informativa

- **Titolare e sede:** DA DEFINIRE, con identità effettiva del gestore.
- **Contatti privacy, richieste assistite, reclami e sicurezza:** DA DEFINIRE. Non pubblicare indirizzi inventati né Mailpit come contatto pubblico.
- **Finalità e basi giuridiche:** proposta tecnica: gestione account/conversazioni, sicurezza, moderazione e assistenza. Associare a ogni trattamento una base valutata dal consulente; non usare un consenso generico per tutto.
- **Destinatari e responsabili:** sviluppo attuale su macchina locale, PostgreSQL, SeaweedFS e Mailpit; futuri hosting/SMTP/supporto da identificare, con accordi e trasferimenti internazionali da valutare.
- **Diritti e modalità:** export/delete in app, rettifica profilo; accesso, opposizione, limitazione, verifica identità, account sospesi e richieste oltre quota richiedono un canale assistito. Definire termini, responsabili e registro delle richieste.
- **Età e destinatari del servizio:** DA DEFINIRE; nessuna raccolta di documenti di identità aggiunta automaticamente.

## Inventario tecnico da allegare alla revisione

| Dato                    | Uso effettivo nel prototipo                            | Conservazione tecnica attuale                                                                                 |
| ----------------------- | ------------------------------------------------------ | ------------------------------------------------------------------------------------------------------------- |
| Email/handle/profilo    | Accesso, presentazione, verifica/reset                 | Fino a cancellazione account; sostituzione immediata dei dati identificativi                                  |
| Password/sessioni/token | Argon2id; hash token, cookie HttpOnly                  | Sessioni fino a scadenza/revoca; cancellate con account/sospensione; pulizia periodica                        |
| Post e relazioni        | Conversazioni, feed cronologico, notifiche             | Testo singolo post eliminato 30 giorni; con account eliminato subito; risposte altrui preservate              |
| Immagini                | Versione PNG/JPEG ricodificata, alt e metadati tecnici | Bozze 24 ore; immagini di post eliminati 30 giorni; account eliminato coda purge immediata                    |
| Segnalazioni            | Esame umano e ricevuta/esito al segnalante             | Aperte fino a revisione; chiuse 180 giorni; dati del segnalante rimossi se elimina account                    |
| Audit                   | Operatore, azione, oggetto, motivazione                | 180 giorni, riferimenti pseudonimi dopo cancellazione; verificare necessità e minimizzazione del testo libero |
| Quote peer/account      | Limitare abusi e richieste costose                     | Finestre di un minuto, pulizia dopo 10 minuti; hash peer non è anonimizzazione forte                          |
| Backup                  | Ripristino verificato                                  | Copie dello script: 30 giorni al successivo backup riuscito; copie manuali separate                           |

Questi tempi sono **scelte tecniche provvisorie**, non durate imposte dalla legge. Report aperti, identificatori residui, eccezioni e motivazioni libere richiedono revisione specifica. Non salvare nei report documenti, credenziali o dati sensibili non necessari.

## Regole community proposte e procedura operativa

Proposta da approvare: vietare spam ripetuto, molestie/minacce, divulgazione non autorizzata di dati personali, impersonificazione e contenuti illeciti. Il gestore deve descrivere esempi, sanzioni proporzionate, criteri e fonti delle regole effettivamente applicate. Il software non decide automaticamente l'illiceità.

Flusso disponibile: utente autenticato segnala un post/profilo con motivo e dettagli; moderatore esamina, registra una motivazione e archivia oppure oscura/sospende; il segnalante può leggere l'esito. Ripristino/riattivazione motivati sono tracciati. Un numero elevato di report non provoca sanzioni automatiche.

Prima del lancio definire un canale di segnalazione anche per soggetti esterni senza account, campi e attestazioni necessari, conferma di ricezione, priorità e gestione degli ordini delle autorità. Definire comunicazione della decisione all'autore con motivazione appropriata, fonti delle regole, durata e possibilità di contestazione. Gli account sospesi non possono usare il login ordinario: assistenza/ricorsi devono restare raggiungibili. Il prototipo non implementa ancora queste procedure pubbliche.

Per contestazioni nell'installazione locale: il gestore raccoglie richiesta e ID del report, un revisore diverso ove possibile riesamina e può registrare `restore_post` o `restore_user`. Nessuna promessa di termine o organo esterno inventato. Stabilire il sistema effettivo e le comunicazioni richieste dopo la valutazione applicabile.

## Backup e incidenti

I backup non vengono ripuliti alla pressione di «Elimina account». Restano privati fino alla propria scadenza; un recupero deve riapplicare cancellazioni successive e revocare accessi prima della riapertura. Non esiste ancora un registro esterno automatico delle cancellazioni. Definire cifratura, autorizzazioni, frequenza, test, gestione di incidenti e obblighi di comunicazione con il consulente. Dettagli tecnici nella [procedura di backup](../operations/backup-restore.md).

Nessun banner di consenso marketing fittizio: nell'anteprima non esistono tracker o pubblicità di terzi. Il cookie di sessione ha uno scopo funzionale di accesso. La disciplina applicabile ai cookie e l'informativa definitiva vanno comunque verificate per il servizio che sarà realmente offerto.
