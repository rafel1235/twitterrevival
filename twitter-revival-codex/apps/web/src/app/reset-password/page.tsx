import { Suspense } from "react";
import { AccountAction } from "@/components/auth/account-actions";
export default function ResetPassword() {
  return (
    <Suspense fallback={<p role="status">Caricamento…</p>}>
      <AccountAction mode="reset" />
    </Suspense>
  );
}
