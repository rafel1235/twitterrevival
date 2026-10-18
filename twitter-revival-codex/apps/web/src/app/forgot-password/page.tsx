import { Suspense } from "react";
import { AccountAction } from "@/components/auth/account-actions";
export default function ForgotPassword() {
  return (
    <Suspense fallback={<p role="status">Caricamento…</p>}>
      <AccountAction mode="forgot" />
    </Suspense>
  );
}
