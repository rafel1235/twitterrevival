# Sprint 2 · Identità e connessioni

Lo Sprint 2 estende il monolite Go/Chi e la stessa applicazione Next.js. Nessun motore post, upload, ricerca per parole chiave o servizio sociale separato. Mailpit è uno strumento SMTP esclusivamente locale: cattura messaggi senza inoltrarli su Internet.

## Identità e profili

I profili richiedono una sessione valida. `GET /api/v1/users/{handle}` restituisce soltanto ID, handle, nome, bio, avatar placeholder, località, sito, data di iscrizione e relazioni/contatori visibili al chiamante. Email, hash e token non sono presenti nel DTO pubblico. Handle ed email restano immutabili; `PATCH /api/v1/users/me` aggiorna esclusivamente il profilo dell’utente derivato dal cookie. Un ID nel body è un campo sconosciuto e viene rifiutato.

Limiti: nome 1–50 code point, bio 0–160, località 0–100. Il client conta con `Array.from`, Go con `utf8.RuneCountInString`, PostgreSQL con `char_length`. Nome/località non ammettono caratteri di controllo; la bio consente newline e tab. I siti sono URL http/https, senza credenziali, massimo 2048 byte; non vengono scaricati dal server. React esegue escaping del testo. Quattro avatar a iniziali (`sage`, `peach`, `lavender`, `sky`), senza upload o immagini esterne. Le nuove schermate usano CSS Modules e i token esistenti.

## Grafo sociale e blocchi

Follows e blocks hanno PK composte, FK con cascade, indici per lettura e paginazione e CHECK contro le relazioni con sé stessi. L’attore di ogni mutazione è sempre l’utente autenticato; l’ID nel percorso è solo il destinatario.

Tutte le mutazioni di una coppia acquisiscono lo stesso advisory lock transazionale PostgreSQL, calcolato sui due UUID canonici ordinati. Questo serializza follow, unfollow, block e unblock anche quando arrivano da istanze/processi diversi. Un blocco elimina i follow in entrambe le direzioni nella stessa transazione e impedisce nuovi follow. Lo sblocco rimuove solo il blocco posseduto dall’attore e non ripristina follow.

Un blocco in uno qualsiasi dei due versi rende profilo ed elenchi dell’altro utente non disponibili (`404`). L’elenco privato `/users/me/blocks` mantiene i dati minimi necessari per sbloccare. Un follow impedito restituisce `403`; relazioni con sé stessi o UUID malformati `422`. Account non attivi non compaiono nei profili/elenchi. Contatori e liste sono filtrati per escludere gli utenti bloccati dal visitatore o che lo bloccano.

Le liste follower, seguiti e bloccati usano cursori Base64URL di timestamp UTC e UUID, ordinati `created_at DESC, id DESC`. Il limite predefinito è 20, massimo 50. I cursori non sono segreti né prove di autorizzazione: ogni query riapplica i filtri di visibilità.

## Verifica email e recupero password

Alla registrazione si accoda una verifica email. Sono disponibili richiesta di un nuovo link, conferma della verifica, richiesta pubblica di recupero e conferma della nuova password. L’accesso e le connessioni non sono ancora subordinati alla verifica email: è una scelta esplicita dell’anteprima locale, da definire prima della beta.

Token CSPRNG di 32 byte, Base64URL; nel database soltanto SHA-256. Scadenza 24 ore per verifica email e 1 ora per reset; uso singolo con purpose vincolato. Un nuovo invio invalida i token precedenti dello stesso tipo; massimo un token emesso al minuto per utente e tipo. L’email è immutabile in questo sprint, quindi il token resta legato allo stesso indirizzo.

Le richieste di recupero restituiscono lo stesso `202` e lo stesso messaggio per indirizzi esistenti e inesistenti. Il percorso HTTP valida soltanto la sintassi e accoda il lavoro: non esegue lookup dell’account o invio SMTP prima di rispondere. La coda è limitata a 32 richieste; un worker interno esegue query ed SMTP con timeout. Richieste rifiutate per coda piena restituiscono `503` senza consultare l’account. Le altre protezioni includono limite di richieste per peer, cooldown per account, origin esatto, Fetch Metadata e header anti-CSRF.

La conferma richiede un POST esplicito dalla pagina: un GET del link non consuma il token. Le pagine e le API impostano `Referrer-Policy: no-referrer`; token esclusi da storage client e log applicativi. La pagina rimuove il token dall’URL dopo il successo.

Per consumare un token si blocca prima la riga utente e poi quella del token, con verifica di scadenza, stato e consumo nella stessa transazione. Il reset aggiorna Argon2id e revoca tutte le sessioni. Anche la creazione di una sessione di login blocca la riga utente e controlla che l’hash password corrisponda a quello verificato: un login iniziato con la vecchia password non può creare una sessione dopo il reset.

SMTP è configurabile con ambiente; credenziali SMTP richiedono STARTTLS, obbligatorio con origin HTTPS. Timeout di connessione e operazione, TLS >=1.2 con verifica certificato. Nessun token restituito dalle API. La coda interna non è durevole: un riavvio può perdere richieste pendenti; gli errori di invio vengono registrati senza indirizzi, password o token. Per questo incremento si richiede nuovamente il link dopo il cooldown. Prima della beta serve una strategia durevole di consegna/retry e retention, senza conservare token in chiaro nel DB.

Riferimenti tecnici: [PostgreSQL advisory locks](https://www.postgresql.org/docs/current/explicit-locking.html#ADVISORY-LOCKS), [OWASP Forgot Password](https://cheatsheetseries.owasp.org/cheatsheets/Forgot_Password_Cheat_Sheet.html), [Mailpit Docker](https://mailpit.axllent.org/docs/install/docker/).

## Migrazioni e upgrade

`002_identity_social` aggiunge avatar_style, follows, blocks e auth_tokens, preservando utenti, profili e sessioni esistenti. Il runner applica tutte le versioni pendenti sotto lo stesso lock e transazione; rifiuta versioni sconosciute e cataloghi non contigui. `-migrate=down` annulla solo l’ultima versione applicata: prima la 002, poi la 001 solo a una seconda esecuzione. Il rollback della 002 elimina connessioni e token, ma conserva account e sessioni; la 001 resta distruttiva per l’autenticazione.

Prima dell’upgrade del database locale è stato salvato un dump PostgreSQL custom sotto `.local/backups/`, escluso da Git. I test distruttivi delle migrazioni usano esclusivamente schemi isolati nel database di test separato.
