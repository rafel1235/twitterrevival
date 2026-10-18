import Link from "next/link";
import styles from "@/components/safety/safety.module.css";
export default function PrivacyPage() {
  return (
    <>
      <header className="feed-header">
        <div>
          <h1>Dati e sicurezza</h1>
          <p>Informazioni sull’anteprima locale</p>
        </div>
      </header>
      <section className={styles.content}>
        <h2>Anteprima di sviluppo</h2>
        <p>
          Questa installazione locale non è un servizio pubblico. Il gestore
          deve completare l’informativa, i contatti e la verifica legale prima
          di un eventuale lancio.
        </p>
        <h2>Dati trattati</h2>
        <p>
          Account, email, credenziali protette, profilo, post, immagini,
          connessioni, notifiche e segnalazioni vengono conservati nel database
          e nello storage locali. Non sono previsti messaggi privati, pubblicità
          o strumenti analitici di terzi. Le immagini caricate vengono
          ricodificate; lo storage conserva la versione elaborata.
        </p>
        <h2>Esportazione e cancellazione</h2>
        <p>
          La beta registra il codice invito in forma protetta e i tempi del
          primo follow, post, interazione e ritorno al settimo giorno. Le
          statistiche aggregate sono riservate ai moderatori; i tuoi dati beta
          sono inclusi nell’esportazione e rimossi quando elimini l’account. Gli
          inviti inutilizzati scaduti o revocati vengono rimossi dopo 30 giorni.
          Le richieste email in coda scadono dopo 24 ore. Il monitor locale
          conserva metriche tecniche aggregate per 7 giorni, senza email,
          indirizzi IP o contenuti.
        </p>
        <p>
          Da{" "}
          <Link href="/settings/account" className="text-button">
            I tuoi dati
          </Link>{" "}
          puoi scaricare un archivio ZIP con JSON e immagini, oppure eliminare
          l’account confermando la password. La cancellazione rimuove subito i
          dati personali dal profilo e il testo dei post; le immagini sono
          eliminate da una coda persistente, normalmente entro un minuto se lo
          storage risponde.
        </p>
        <h2>Conservazione</h2>
        <p>
          Le bozze non pubblicate scadono dopo 24 ore. Il testo dei singoli post
          eliminati viene cancellato dopo 30 giorni; l’eliminazione dell’account
          lo cancella subito. Segnalazioni chiuse e azioni di moderazione hanno
          una finestra tecnica di 180 giorni. Le segnalazioni aperte richiedono
          revisione umana. Identificatori pseudonimi necessari a collegare le
          risposte possono restare nei record rimossi.
        </p>
        <p>
          I backup sono separati dai dati attivi: il comando locale conserva
          copie private e applica una scadenza di 30 giorni soltanto alle copie
          create nella propria cartella. Un ripristino deve riapplicare le
          cancellazioni successive alla copia. Nessuna promessa di cancellazione
          immediata dai backup.
        </p>
        <h2>Segnalazioni e blocchi</h2>
        <p>
          Puoi segnalare un post o un profilo e consultare l’esito nelle
          impostazioni. Il blocco nasconde i contenuti reciproci e rimuove i
          follow. Le decisioni dei moderatori richiedono una motivazione e
          vengono registrate.
        </p>
        <p>
          Per l’anteprima locale, le richieste assistite e le contestazioni
          vanno rivolte a chi gestisce questa installazione. Il canale pubblico
          e la gestione dei reclami devono essere definiti prima del lancio.
        </p>
      </section>
    </>
  );
}
