# Backup e prova di ripristino locale

Il backup comprende **PostgreSQL e l'intero volume SeaweedFS**. Un solo dump SQL non salva le immagini. Nessun cloud o servizio a pagamento. Script PowerShell dalla root del progetto, Docker Desktop avviato e immagini Sprint 5 già compilate:

```powershell
docker compose up -d --build --wait
./scripts/backup-verify.ps1
```

Lo script trova Docker anche nel percorso Desktop standard dell'utente Windows. Non modifica PATH globale o credenziali. Riutilizza `.env` tramite Compose senza mostrarlo, non lo include nel backup. In Windows limita l'ACL della cartella a chi esegue il comando.

## Cosa viene verificato

1. Ferma soltanto l'API per impedire nuove scritture applicative. Nessun test o client S3 diretto deve scrivere durante questa fase. Calcola un manifest con conteggi/digest SHA-256 di ogni tabella e dei byte di ogni oggetto nel bucket applicativo.
2. `pg_dump -Fc --no-owner` del database esistente, senza cancellarlo. Ferma brevemente SeaweedFS e archivia il volume montato in sola lettura.
3. Nel `finally` riavvia storage e API anche in caso di errore. Il sito può rispondere 502 durante la breve pausa. Gli altri servizi e i volumi originali rimangono presenti.
4. Crea progetto Compose casuale `radura-restore-XXXXXXXX`, rete separata, database vuoto e volume S3 nuovo. Nessuna porta esposta. Ripristina dump e archivio storage con le stesse versioni e credenziali locali.
5. Ricalcola digest/conteggi mediante il binario API della copia. Il confronto comprende **tutte le righe e i byte delle immagini**, non soltanto l'healthcheck o l'elenco del dump. Differenze/errori fanno fallire il comando.
6. Scrive `verified.json` solo a confronto riuscito. Rimuove esclusivamente container/rete/volumi del progetto temporaneo, verificando nomi e isolamento. Non effettua mai un restore sul database attivo.

Copie in `.local/backups/sprint5/YYYYMMDDTHHMMSSZ_XXXXXXXX/`: `database.dump`, `media.tar`, `manifest.json`, `verified.json`. Tutto ignorato da Git. I manifest contengono solo conteggi e digest; dump/tar contengono dati personali e credenziali hashate: conservarli privatamente, cifrare disco/copie esterne e non allegarli alle issue. Il file `.env` e la versione dell'immagine sono da custodire separatamente per il disaster recovery.

## Conservazione e recupero operativo

Lo script elimina le proprie cartelle datate con più di 30 giorni **dopo una nuova verifica riuscita**. Valida percorsi e rifiuta link, sottocartelle o contenuti inattesi; non tocca i dump manuali `before-sprint*.dump`. Se lo script non viene eseguito, la retention dei backup non avanza: nessun scheduler implicito è stato installato. Il gestore deve pianificare frequenza e controllo esiti; copie manuali richiedono eliminazione separata.

La procedura è una prova riproducibile, non una politica aziendale completa di disaster recovery. Per un recupero effettivo: scegliere un backup con ricevuta valida, verificare gli hash dei due archivi, mantenere il servizio isolato, ripristinare in volumi nuovi e verificare nuovamente il manifest prima di collegare il frontend. Conservare i volumi precedenti fino alla verifica; **non usare `docker compose down -v` sul progetto attivo**.

Prima di rendere utilizzabile una copia vecchia, il gestore deve riapplicare cancellazioni e revoche intervenute dopo la data del backup, ed eseguire manutenzione. Non esiste ancora un registro esterno durevole di queste cancellazioni: il ripristino pubblico rimane una procedura assistita da completare prima della beta. Nessuna garanzia che la cancellazione dell'account rimuova istantaneamente le copie storiche.

Obiettivi RPO/RTO, cifratura automatica, rotazione chiavi, backup fuori macchina e simulazione di perdita del disco non sono introdotti da questo sprint. La prova locale effettiva e i numeri sono nel registro di implementazione.
