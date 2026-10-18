"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { useState } from "react";
import { useSession } from "@/components/auth/session-provider";
import { Icon } from "./icon";
import { UserAvatar } from "@/components/profile/shared";
import { SearchBox } from "@/components/discovery/search-view";
import { useUnreadNotifications } from "@/lib/notifications";

export const productName = process.env.NEXT_PUBLIC_PRODUCT_NAME || "Radura";

function AccountPanel() {
  const { user, loading, error, refresh, logout } = useSession();
  const [pending, setPending] = useState(false);
  const [logoutError, setLogoutError] = useState("");
  async function signOut() {
    setPending(true);
    setLogoutError("");
    try {
      await logout();
    } catch (err) {
      setLogoutError(
        err instanceof Error ? err.message : "Uscita non riuscita.",
      );
    } finally {
      setPending(false);
    }
  }
  return (
    <div className="account-panel">
      {loading ? (
        <p className="muted" role="status">
          Verifica sessione…
        </p>
      ) : user ? (
        <>
          <div className="account-identity">
            <UserAvatar
              name={user.display_name}
              style={user.avatar_style}
              small
            />
            <div>
              <strong data-testid="session-name">{user.display_name}</strong>
              <span className="muted">@{user.handle}</span>
            </div>
          </div>
          <button className="text-button" onClick={signOut} disabled={pending}>
            {pending ? "Uscita…" : "Esci"}
          </button>
        </>
      ) : (
        <>
          <p className="account-prompt">Trova il tuo posto.</p>
          <Link className="button primary" href="/register">
            Crea un account
          </Link>
          <Link className="account-login" href="/login">
            Hai già un account? Accedi
          </Link>
        </>
      )}
      {error && (
        <div className="session-error" role="alert">
          <p>{error}</p>
          <button className="text-button" onClick={() => void refresh()}>
            Riprova
          </button>
        </div>
      )}
      {logoutError && (
        <p role="alert" className="error-text">
          {logoutError}
        </p>
      )}
    </div>
  );
}

export function AppShell({ children }: { children: React.ReactNode }) {
  const pathname = usePathname();
  const { user } = useSession();
  const unread = useUnreadNotifications(user?.id);
  const bell = (
    <span className="notification-icon">
      <Icon name="bell" />
      {unread > 0 && (
        <span
          className="notification-badge"
          aria-label={`${unread} da leggere`}
        >
          {unread > 99 ? "99+" : unread}
        </span>
      )}
    </span>
  );
  return (
    <div className="app-shell">
      <a className="skip-link" href="#main">
        Vai al contenuto
      </a>
      <aside className="left-column">
        <Link href="/" className="brand" aria-label={`${productName}, home`}>
          <span className="brand-mark">
            <Icon name="leaf" size={29} />
          </span>
          <span className="brand-name">
            {productName}
            <small>uno spazio per le tue idee</small>
          </span>
        </Link>
        <nav aria-label="Navigazione principale" className="desktop-nav">
          <Link
            href="/"
            className={`nav-item ${pathname === "/" ? "active" : ""}`}
            aria-current={pathname === "/" ? "page" : undefined}
          >
            <Icon name="home" />
            <span>Home</span>
          </Link>
          <Link
            className={`nav-item ${pathname === "/search" ? "active" : ""}`}
            href="/search"
            aria-current={pathname === "/search" ? "page" : undefined}
          >
            <Icon name="search" />
            <span>Esplora</span>
          </Link>
          <Link
            className={`nav-item ${pathname === "/notifications" ? "active" : ""}`}
            href="/notifications"
            aria-label="Notifiche"
            aria-current={pathname === "/notifications" ? "page" : undefined}
          >
            {bell}
            <span>Notifiche</span>
          </Link>
          <Link
            href="/people"
            className={`nav-item ${pathname === "/people" ? "active" : ""}`}
            aria-label="Persone"
          >
            <Icon name="search" />
            <span>Persone</span>
          </Link>
          <Link
            className={`nav-item ${pathname.startsWith("/u/") ? "active" : ""}`}
            href={user ? `/u/${user.handle}` : "/login"}
            aria-label="Il tuo profilo"
          >
            <Icon name="user" />
            <span>Profilo</span>
          </Link>
          <Link
            className={`nav-item ${pathname.startsWith("/settings/") ? "active" : ""}`}
            href="/settings/profile"
            aria-label="Impostazioni profilo"
          >
            <Icon name="settings" />
            <span>Impostazioni</span>
          </Link>
        </nav>
        <div className="sprint-note">
          <span className="status-dot" /> Sprint 4 · Nuove prospettive
        </div>
        <AccountPanel />
        <p className="independent-note">
          Un progetto indipendente.
          <br />
          Fatto per conversare, con calma.
        </p>
      </aside>
      <main id="main" className="main-column">
        {children}
      </main>
      <aside className="right-column" aria-label="Informazioni sulla community">
        <SearchBox compact />
        <section className="welcome-card">
          <span className="eyebrow">
            BENVENUTI IN {productName.toUpperCase()}
          </span>
          <div className="welcome-art" aria-hidden="true">
            <span />
            <span />
            <span />
            <Icon name="leaf" size={58} />
          </div>
          <h2>
            Meno rumore.
            <br />
            Più conversazioni.
          </h2>
          <p>
            Un piccolo spazio aperto per condividere pensieri e incontrare nuove
            prospettive.
          </p>
          <Link href="/register" className="button primary">
            Entra nella radura <Icon name="arrow" size={17} />
          </Link>
        </section>
        <section className="info-card">
          <h2>Un passo alla volta</h2>
          <div className="milestone">
            <span className="milestone-number">01</span>
            <div>
              <strong>Mettiamo radici</strong>
              <p>Account e accesso · Completato</p>
            </div>
          </div>
          <div className="milestone">
            <span className="milestone-number">02</span>
            <div>
              <strong>Incontriamoci</strong>
              <p>Profili e connessioni · Completato</p>
            </div>
          </div>
          <div className="milestone">
            <span className="milestone-number">03</span>
            <div>
              <strong>Inizia la conversazione</strong>
              <p>Post e timeline reali · Completato</p>
            </div>
          </div>
          <div className="milestone">
            <span className="milestone-number current">04</span>
            <div>
              <strong>Nuove prospettive</strong>
              <p>Immagini, ricerca e notifiche · Sprint attuale</p>
            </div>
          </div>
        </section>
        <p className="right-footer">
          Anteprima di sviluppo · La timeline autenticata contiene post reali.
          Nessuna affiliazione con Twitter o X.
        </p>
      </aside>
      <nav className="mobile-nav" aria-label="Navigazione mobile">
        <Link
          href="/"
          aria-label="Home"
          aria-current={pathname === "/" ? "page" : undefined}
        >
          <Icon name="home" />
        </Link>
        <Link
          href="/search"
          aria-label="Esplora"
          aria-current={pathname === "/search" ? "page" : undefined}
        >
          <Icon name="search" />
        </Link>
        <Link
          href="/notifications"
          aria-label="Notifiche"
          aria-current={pathname === "/notifications" ? "page" : undefined}
        >
          {bell}
        </Link>
        <Link href="/settings/profile" aria-label="Impostazioni profilo">
          <Icon name="settings" />
        </Link>
        <Link
          href={user ? `/u/${user.handle}` : "/login"}
          aria-label={user ? "Il tuo profilo" : "Account"}
        >
          <Icon name="user" />
        </Link>
      </nav>
    </div>
  );
}
