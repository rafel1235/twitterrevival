import { BlockedUsers } from "@/components/profile/connections";
import { SettingsNav } from "@/components/safety/settings-nav";
export default function BlocksPage() {
  return (
    <>
      <SettingsNav />
      <BlockedUsers />
    </>
  );
}
