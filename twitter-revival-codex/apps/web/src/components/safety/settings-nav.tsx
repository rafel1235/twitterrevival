"use client";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { useSession } from "@/components/auth/session-provider";
import styles from "./safety.module.css";

export function SettingsNav() {
  const path = usePathname();
  const { user } = useSession();
  const links = [
    ["/settings/profile", "Profilo"],
    ["/settings/blocks", "Blocchi"],
    ["/settings/account", "I tuoi dati"],
    ["/settings/reports", "Segnalazioni"],
  ];
  if (user?.role === "moderator") links.push(["/moderation", "Moderazione"]);
  return (
    <nav className={styles.nav} aria-label="Impostazioni e sicurezza">
      {links.map(([href, label]) => (
        <Link
          key={href}
          href={href}
          aria-current={path === href ? "page" : undefined}
        >
          {label}
        </Link>
      ))}
    </nav>
  );
}
