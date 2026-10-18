import { SearchView } from "@/components/discovery/search-view";
export default async function SearchPage({
  searchParams,
}: {
  searchParams: Promise<{ q?: string | string[]; type?: string | string[] }>;
}) {
  const params = await searchParams;
  return (
    <SearchView
      query={typeof params.q === "string" ? params.q : ""}
      type={typeof params.type === "string" ? params.type : "posts"}
    />
  );
}
