# Matrice UI Sprint 7

Incremento originale Radura dedicato a usabilità/accessibilità/affidabilità; nessun asset, codice o dato esterno copiato. Nessuna nuova funzione sociale o dipendenza cloud.

| Area        | Comportamento verificabile                                                                                                                         | Prova                                                              |
| ----------- | -------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------ |
| Shell       | Nomi anche con label nascoste su tablet, pagina corrente, skip link con focus nel main                                                             | Tastiera e axe desktop/tablet/mobile                               |
| Testo       | Contrasto dei testi secondari, reflow a 320 px, campi utilizzabili con testo raddoppiato; brand, voci di navigazione e titolo laterale senza tagli | axe, overflow della pagina e dei singoli contenitori, screenshot   |
| Form        | Errori associati ai campi, focus sul primo errore oppure riepilogo, valori conservati                                                              | Login con errori iniettati e validazione profilo reale nel browser |
| Feed        | Guasto transitorio conserva i dati indicandone lo stato; errore di autorizzazione li rimuove                                                       | 503 e 403 iniettati sul feed reale                                 |
| Paginazione | Riprova il cursore fallito, senza duplicare/rimuovere post precedenti                                                                              | Metadati di cursore sintetici, contenuti provenienti da PostgreSQL |
| Composer    | Bozza mantenuta dopo connessione interrotta, nessun invio automatico ripetuto                                                                      | Errore browser e successiva pubblicazione reale unica              |
| Notifiche   | Lista mantenuta dopo errore di aggiornamento                                                                                                       | Follow fra account sintetici reali, 503 browser, recupero          |
| Sessione    | Errore di verifica distinto dalla demo; scadenza/revoca annunciata e dati privati rimossi                                                          | Rete e 401 simulati nei soli contesti di test                      |
| Pagina      | Boundary locale con tentativo di recupero e link home                                                                                              | Payload errato iniettato e recupero sul backend reale              |

Gli audit controllano le regole automatiche axe per WCAG 2/2.1/2.2 A/AA nelle pagine e negli stati testati, senza esclusioni delle regole individuate. Non sono una certificazione né una verifica umana con lettori di schermo. I risultati JSON sono allegati al report Playwright locale e non contengono dati di piloti esterni.
