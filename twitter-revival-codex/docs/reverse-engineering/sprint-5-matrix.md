# Matrice UI Sprint 5

Provenienza: componenti, brand, CSS e icone originali già presenti; riferimento storico limitato alla struttura a tre colonne e al feed ~600 px. Nessun asset o sorgente Twitter/X recuperato. Nuove schermate progettate sui flussi reali di questo monolite.

| Schermata              | Elementi e stati                                                                                       | Responsive / verifica                                                                    |
| ---------------------- | ------------------------------------------------------------------------------------------------------ | ---------------------------------------------------------------------------------------- |
| Post/profilo altrui    | Disclosure «Segnala», motivo, dettagli, invio reale, ricevuta, errore e pending                        | Form nella colonna; label e summary da tastiera; assente sui propri oggetti              |
| Segnalazioni personali | Stato, dettaglio, motivazione, pagine a cursore; loading/empty/retry                                   | Navigazione impostazioni a capo, testo lungo spezzato                                    |
| Moderazione            | Gate ruolo, coda con filtri, prova testuale, decisione con motivo, stato aggiornato                    | Una colonna operativa anche su mobile, controlli con label, nessuna tabella larga        |
| Audit                  | Azioni, motivo, ID oggetto/operatore, data, pagina successiva                                          | Testo in wrapping; solo ruolo autorizzato                                                |
| I tuoi dati            | Password, archivio ZIP, errori, caricamento, area cancellazione con conferma ELIMINA e risultato reale | Pulsanti e campi nell'ampiezza della colonna; sequenza tastiera lineare                  |
| Dati e sicurezza       | Informazioni dell'anteprima e tempi tecnici, link impostazioni                                         | Pagina pubblica leggibile senza login; non presentata come informativa legale definitiva |

Playwright verifica desktop 1440×900, tablet 768×1024, mobile 390×844. Screenshot moderazione/account nel report di test. Nessun mock API o ruolo impostato nello stato client: il test crea un proprio account e usa la CLI locale di bootstrap, revocando il ruolo dopo la prova.
