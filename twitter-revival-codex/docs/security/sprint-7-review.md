# Review tecnica Sprint 7

Perimetro: beta locale e consolidamento dell’interfaccia. Nessuna nuova tabella, API o autenticazione alternativa. Le autorizzazioni, l’origine esatta e la protezione CSRF restano nel backend.

- Richieste con deadline anche durante la lettura del corpo, cancellazione del chiamante preservata, errori HTTP/HTML/JSON non validi presentati senza dettagli tecnici. Valori di campo non testuali ignorati per evitare rendering di oggetti arbitrari.
- Nessun retry automatico delle mutazioni. Un’interruzione dopo il commit del server è ambigua: il messaggio invita a verificare l’esito. Il controllo duplicati del backend rimane attivo.
- Cancellazione e sequenza delle richieste impediscono a un aggiornamento precedente di sostituire quello recente. Le liste mantengono i dati su guasti transitori, ma li rimuovono su 401/403/404. La scadenza/revoca della sessione rimuove le viste autenticate e mostra il percorso per accedere nuovamente.
- La bozza del composer resta soltanto in memoria mentre il componente rimane montato. Nessun testo privato aggiunto a localStorage/sessionStorage, nessuna nuova telemetria. Navigazione, ricaricamento, scadenza sessione o errore di rendering possono perdere una bozza non pubblicata.
- L’error boundary riguarda le pagine sotto il layout; non costituisce recupero da ogni guasto del layout, del processo Next o del server. Test di recupero eseguiti con payload errato iniettato nel browser.
- I test backend completi usano un progetto Docker casuale con PostgreSQL e S3 effimeri, credenziali sintetiche, nessuna porta host e sorgente in sola lettura. Preflight controlla mount, reti e credenziali; cleanup limitato al progetto appena creato. Le cache Go condivise contengono dipendenze/build, non credenziali applicative.
- I nuovi test browser creano account nelle coorti di test e li eliminano tramite API con la propria password. I guasti HTTP/rete sono circoscritti al contesto Playwright. L’eccezione TLS vale soltanto per localhost; la suite HTTPS verifica separatamente la CA locale.

## Dipendenze

Aggiunto soltanto `@axe-core/playwright` 4.14.0 fra le dipendenze di sviluppo. L’audit di produzione del 18 ottobre 2026 (`npm audit --omit=dev`) riporta zero vulnerabilità note. L’audit completo riporta cinque segnalazioni high nella catena dev `eslint-config-next → @next/eslint-plugin-next → fast-glob → micromatch → braces`, riconducibili all’advisory [GHSA-vfj7-8cjw-p6xm](https://github.com/advisories/GHSA-vfj7-8cjw-p6xm). Il registro non indica una versione corretta; il suggerimento npm comporta un downgrade principale della configurazione Next e non è stato applicato. I pattern di lint provengono dal repository, non dagli utenti del social. Questo è un limite noto, non una dichiarazione generale di assenza di vulnerabilità.

Restano i limiti degli sprint precedenti: una macchina locale, quote peer condivise dietro Next, valutazione umana/legale prima dell’apertura, backup fuori macchina e osservazione di piloti reali ancora da predisporre. Dominio e hosting sono rimandati esplicitamente.
