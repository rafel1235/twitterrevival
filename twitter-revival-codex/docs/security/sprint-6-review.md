# Review tecnica della beta locale

Revisione del codice e test automatici, non penetration test indipendente. Perimetro: macchina locale, nessuna esposizione pubblica autorizzata.

| Rischio                       | Controllo implementato e verifica                                                            | Limite operativo                                                                              |
| ----------------------------- | -------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------- |
| Accesso senza invito          | Preflight e ricontrollo sotto lock, hash CSPRNG, email vincolata, consumo atomico            | Coorti di test consentite soltanto da CLI e origine locale; accesso DB/CLI resta privilegiato |
| Riuso/capienza                | Concorrenza consumo e ultimo posto, rollback conflitto handle, revoca idempotente            | Invito riserva un posto fino a scadenza/revoca; membri precedenti fuori coorte                |
| Gestione non autorizzata      | Ruolo/stato dal DB, Origin/CSRF e quote; test anonimo/member/moderatore                      | Il primo ruolo è un’operazione locale esplicita                                               |
| Esposizione token             | Token mostrato una volta, mai nella lista; frammento rimosso subito                          | Chi riceve il link deve custodirlo; niente catture schermo/log del codice valido              |
| Perdita email al riavvio      | Intenti persistenti, lease/ack/retry, test SMTP fallito e worker nuovo                       | SMTP almeno una volta, 5 tentativi, massimo 32 pendenti, scadenza 24h; Mailpit solo locale    |
| Informazioni nel monitor      | Bearer dedicato, endpoint interno, route template, nessun URL/query/utente, test del payload | Prometheus loopback privo di login; utenti della macchina devono essere attendibili           |
| Allarmi falsi per navigazione | Annullamento client 499, timeout 504; panic vero ancora 500 e contato                        | I client disconnessi possono non ricevere alcuna risposta                                     |
| TLS e cookie                  | CA interna, hostname/chain validati da Node, cookie __Host Secure, Origin HTTPS              | CA Windows manuale; chiavi private nel volume Caddy; nessuna verifica con CA pubblica         |
| Nuovi dati personali          | Export dei propri indicatori, cancellazione inviti/email/membership, test dedicato           | Backup storici separati; retention e basi del pilotaggio da validare prima dell’esterno       |
| Interpretazione D7            | Coorti test escluse, finestra completa prima del denominatore, null iniziale                 | Polling autenticato conta; nessun risultato di esseri umani disponibile                       |
| Pressione sul servizio        | Carico con soglie esplicite e quote reali, report senza credenziali                          | Una macchina, carico misto moderato, nessuna prova di saturazione distribuita                 |

I log applicativi continuano a contenere classi tecniche, senza errori driver grezzi. Caddy non abilita access log con URL; i token verifica/reset già esistenti restano nei messaggi privati Mailpit. Segreti `.env`, `.local/metrics-token`, dump e chiavi CA esclusi da Git. Il certificato pubblico esportato è anch’esso sotto `.local`.

Rimangono i limiti della [review Sprint 5](sprint-5-review.md): canali pubblici e valutazione legale da completare, quote peer condivise dietro Next, assistenza per export grandi, ripristino assistito con cancellazioni successive al backup, nessun test indipendente. Non sono stati attivati tunnel, inoltri Internet o notifiche a terzi.
