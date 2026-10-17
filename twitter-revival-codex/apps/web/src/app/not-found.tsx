import Link from "next/link";
export default function NotFound() {
  return (
    <section className="auth-card">
      <h1>Qui non c’è ancora nulla.</h1>
      <p>La pagina richiesta non esiste.</p>
      <Link href="/" className="button primary">
        Torna alla home
      </Link>
    </section>
  );
}
