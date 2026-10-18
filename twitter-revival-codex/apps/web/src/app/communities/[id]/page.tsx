import { CommunityDetail } from "@/components/communities/community-detail";

export default async function CommunityPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = await params;
  return <CommunityDetail id={id} />;
}
