"use client";

import Link from "next/link";

export default function ErrorPage({
  reset,
}: {
  error: Error & { digest?: string };
  reset: () => void;
}) {
  return (
    <section className="recovery-state" aria-labelledby="page-error-title">
      <h1 id="page-error-title">Questa pagina non si è caricata</h1>
      <p role="alert">
        Si è verificato un problema. Puoi riprovare o tornare alla home.
      </p>
      <button type="button" className="button primary" onClick={reset}>
        Riprova pagina
      </button>
      <Link className="text-button" href="/">
        Torna alla home
      </Link>
    </section>
  );
}
