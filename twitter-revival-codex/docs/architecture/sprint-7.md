# Sprint 7 — usabilità, accessibilità e affidabilità locale

Ambito scelto dall’utente il 18 ottobre 2026. La specifica originale termina allo Sprint 6; questo incremento consolida la beta locale. Dominio, VPS e servizi gestiti sono rimandati. Nessuna nuova funzione sociale, dipendenza cloud o modifica al modello dati.

## Criteri di completamento

- Navigazione con nomi accessibili anche su tablet, salto al contenuto con focus effettivo, indicazione della pagina corrente coerente e layout verificato a 320 px.
- Errori di validazione collegati ai campi, focus sul primo errore e riscontri leggibili durante l’invio.
- Richieste con attesa limitata, cancellazione alla navigazione, risposte obsolete ignorate ed errori comprensibili. Nessun retry automatico delle mutazioni: un timeout non prova che la scrittura sia fallita.
- Feed e notifiche già caricati conservati durante guasti transitori; retry della pagina fallita senza perdere il cursore. Dati non autorizzati eliminati alla scadenza della sessione.
- Controlli automatici axe e prove Playwright su desktop/tablet/mobile, insieme ai controlli del repository. Le prove automatiche non costituiscono certificazione di conformità WCAG o collaudo umano con tecnologie assistive.

## Verifica iniziale

Al commit iniziale `7c8df05`, Git pulito. Audit Chromium/axe della beta HTTPS locale: link Home/Esplora privi di nome a 768 px; nome accessibile del brand diverso dal testo visibile a 1440 px; contrasto 4,47:1 dei numeri informativi contro il minimo 4,5:1. Nessun overflow sulle quattro pagine pubbliche controllate a 1440, 768, 390 e 320 px.

Esiti finali e limiti in `docs/implementation-status.md`.
