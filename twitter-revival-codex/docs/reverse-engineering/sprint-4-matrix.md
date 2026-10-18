# Matrice UI · Sprint 4

Continua il design originale Radura. Ricerca compatta, card con immagine ed elenco attività derivano dai requisiti e dai token esistenti: dettagli indipendenti, non osservazioni attribuite a pagine storiche. Nessun codice, logo, fotografia o dato copiato da Twitter/X.

| Schermata         | Elementi / stati                                                                     | Responsive e accessibilità                                                               | File                                                   |
| ----------------- | ------------------------------------------------------------------------------------ | ---------------------------------------------------------------------------------------- | ------------------------------------------------------ |
| Composer immagini | JPEG/PNG, descrizione, upload/preview reali, rimozione, bozze recuperabili e scadute | Label, pending/error, pubblicazione impedita con file non caricato; max quattro allegati | `composer.tsx`, `draft-images.tsx`                     |
| Post con immagini | API autenticata, alt text, apertura originale, indisponibile                         | Griglia fluida, dimensioni riservate, nessun URL S3 pubblico o overflow                  | `attachments.tsx`, `posts.module.css`                  |
| Ricerca           | Form GET, tab Post/Persone, risultati paginati, empty/loading/error/retry            | Campo fluido, ricerca rapida nella sidebar desktop                                       | `search-view.tsx`, `app/search/page.tsx`               |
| Notifiche         | Cinque tipi, attore, data, link, unread, lettura singola/tutte                       | Righe compatte, indicatore laterale e contrasto; bottoni a capo mobile                   | `notifications-view.tsx`, `discovery.module.css`       |
| Navigazione       | Ricerca/notifiche attive, badge API, mobile a cinque voci                            | Badge accessibile, nessun conteggio simulato                                             | `app-shell.tsx`, `lib/notifications.ts`, `globals.css` |

Fixture `apps/web/tests/fixtures/radura-landscape.png`: raster originale 600×360 creato proceduralmente con Go (gradienti, sole circolare e colline sinusoidali), solo per test. Nessun asset esterno, fotografia di terzi o persona reale.

Screenshot a 1440×900, 768×1024 e 390×844: risultati con immagine e notifiche, desktop/mobile ispezionati. La home anonima conserva la demo dichiarata; le nuove funzioni richiedono accesso. Esiti e artifact in `docs/implementation-status.md`.
