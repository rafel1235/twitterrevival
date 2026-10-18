export type Profile = {
  id: string;
  handle: string;
  display_name: string;
  bio: string;
  avatar_style: string;
  location_text: string;
  website_url: string;
  created_at: string;
  is_self: boolean;
  following: boolean;
  followed_by: boolean;
  followers_count: number;
  following_count: number;
};
export type Connection = Pick<
  Profile,
  "id" | "handle" | "display_name" | "avatar_style"
>;
export type ConnectionPage = { users: Connection[]; next_cursor?: string };
