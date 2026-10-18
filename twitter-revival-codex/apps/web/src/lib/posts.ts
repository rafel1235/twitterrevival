export type Author = {
  id: string;
  handle: string;
  display_name: string;
  avatar_style: string;
};
export type PostPreview = {
  id: string;
  author: Author;
  body: string;
  created_at: string;
  updated_at: string;
  media: Attachment[];
  community_id?: string;
  community_name?: string;
};
export type Attachment = {
  id: string;
  url: string;
  mime_type: string;
  bytes: number;
  width: number;
  height: number;
  alt_text: string;
};
export type Post = PostPreview & {
  reply_to_id?: string;
  quote_of_id?: string;
  reply_to: PostPreview | null;
  quote_of: PostPreview | null;
  is_own: boolean;
  likes_count: number;
  reposts_count: number;
  replies_count: number;
  quotes_count: number;
  liked: boolean;
  reposted: boolean;
};
export type PostEvent = {
  id: string;
  created_at: string;
  post: Post;
  reposted_by?: Author;
};
export type PostPage = { items: PostEvent[]; next_cursor?: string };
export function postsChanged() {
  window.dispatchEvent(new Event("radura:posts-changed"));
}
// Unicode White_Space matches Go strings.TrimSpace, including NEL but not BOM.
export function normalizePostBody(body: string) {
  return body.replace(/^\p{White_Space}+|\p{White_Space}+$/gu, "");
}
export function postBodyError(body: string) {
  body = normalizePostBody(body);
  const count = Array.from(body).length;
  const invalidControl = Array.from(body).some((character) => {
    const code = character.codePointAt(0)!;
    return (
      (code < 32 && character !== "\n" && character !== "\t") ||
      (code >= 127 && code <= 159)
    );
  });
  return count < 1 || count > 280 || invalidControl
    ? "Scrivi da 1 a 280 caratteri Unicode, senza caratteri di controllo."
    : "";
}
