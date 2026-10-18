import { Suspense } from "react";
import { AccountAction } from "@/components/auth/account-actions";
export default function VerifyEmail() {
  return (
    <Suspense fallback={<p role="status">Caricamento…</p>}>
      <AccountAction mode="verify" />
    </Suspense>
  );
}
