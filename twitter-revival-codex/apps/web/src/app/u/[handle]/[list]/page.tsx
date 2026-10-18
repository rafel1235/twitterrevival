import { notFound } from "next/navigation";
import { ProfileView } from "@/components/profile/profile-view";
export default async function ConnectionsPage({
  params,
}: {
  params: Promise<{ handle: string; list: string }>;
}) {
  const { handle, list } = await params;
  if (list !== "followers" && list !== "following") notFound();
  return <ProfileView handle={handle} kind={list} />;
}
