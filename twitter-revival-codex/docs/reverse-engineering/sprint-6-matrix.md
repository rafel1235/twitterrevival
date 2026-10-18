# Matrice UI Sprint 6

Schermate originali Radura; nessun asset o codice di Twitter/X. Le tre colonne e il feed circa 600px riprendono soltanto il linguaggio di layout già documentato. Pannello beta e inviti sono progettati per questo prodotto.

| Schermata     | Elementi e stati                                                                                                                                                    | Responsive e verifica                                                                |
| ------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------ |
| Registrazione | Codice invito password, email vincolata, frammento rimosso, attesa config, errore/riprova, invio, pausa ingressi                                                    | Form reale nei tre viewport; registrazione con invito e navigazione                  |
| Beta e inviti | Gate sessione/ruolo, caricamento/errore, sei metriche, D7 non misurabile, capienza, pausa/riapertura, creazione link una volta, ultimi 100, revoca, stato operativo | Card a due colonne nel feed; inviti e form senza overflow; E2E desktop/tablet/mobile |
| Shell         | Marchio originale, indicazione beta privata e milestone 6                                                                                                           | Sidebar desktop, top bar e navigazione mobile esistenti                              |
| HTTPS         | Stesse pagine reali, cookie sicuro e stato autenticato                                                                                                              | Screenshot home/conversazione e check overflow nei tre viewport                      |
| Privacy       | Indicatori beta, code email e tempi del monitor locale                                                                                                              | Test di accessibilità/navigazione già esistenti                                      |

Screenshot nei risultati Playwright, ignorati da Git. Esaminati desktop e mobile del pannello: testi leggibili, form nel feed, colonne coerenti e navigazione mobile presente. Caricamento/errori e azioni disabilitate durante mutazioni; nessun dato pilota inventato. I conteggi mostrati durante i test appartengono esclusivamente a fixture sintetiche e vengono rimossi alla cancellazione del relativo account.
