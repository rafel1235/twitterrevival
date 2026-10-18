type Name =
  | "home"
  | "search"
  | "bell"
  | "user"
  | "community"
  | "settings"
  | "spark"
  | "reply"
  | "repeat"
  | "heart"
  | "share"
  | "leaf"
  | "image"
  | "arrow";
const paths: Record<Name, string> = {
  home: "M3 10.5 12 3l9 7.5V21h-6v-7H9v7H3Z",
  search: "M21 21l-5-5M18 10.5a7.5 7.5 0 1 1-15 0 7.5 7.5 0 0 1 15 0Z",
  bell: "M18 8a6 6 0 0 0-12 0c0 7-3 7-3 9h18c0-2-3-2-3-9ZM10 21h4",
  user: "M20 21v-2a6 6 0 0 0-6-6h-4a6 6 0 0 0-6 6v2M16 6a4 4 0 1 1-8 0 4 4 0 0 1 8 0Z",
  community:
    "M16 21v-2a5 5 0 0 0-5-5H8a5 5 0 0 0-5 5v2M13 6a3.5 3.5 0 1 1-7 0 3.5 3.5 0 0 1 7 0ZM17 3a3.5 3.5 0 0 1 0 7M21 21v-2a5 5 0 0 0-4-4.9",
  settings:
    "M12 3v3m0 12v3M3 12h3m12 0h3M5.6 5.6l2.1 2.1m8.6 8.6 2.1 2.1M5.6 18.4l2.1-2.1m8.6-8.6 2.1-2.1M16 12a4 4 0 1 1-8 0 4 4 0 0 1 8 0Z",
  spark: "m12 3 2.5 6.5L21 12l-6.5 2.5L12 21l-2.5-6.5L3 12l6.5-2.5Z",
  reply: "M21 11a9 9 0 0 1-9 9H4l1.5-4A9 9 0 1 1 21 11Z",
  repeat:
    "m17 2 4 4-4 4M3 11V8a2 2 0 0 1 2-2h16M7 22l-4-4 4-4m14-1v3a2 2 0 0 1-2 2H3",
  heart:
    "M20.8 4.6a5.5 5.5 0 0 0-7.8 0l-1 1-1-1a5.5 5.5 0 0 0-7.8 7.8L12 21l8.8-8.6a5.5 5.5 0 0 0 0-7.8Z",
  share: "M12 16V3m-5 5 5-5 5 5M4 13v8h16v-8",
  leaf: "M5 19C2 9 9 3 21 3c0 12-6 19-16 16Zm0 0L16 8",
  image: "M3 3h18v18H3Zm0 14 5-5 4 4 3-3 6 6M8 8h.01",
  arrow: "M4 12h16m-6-6 6 6-6 6",
};
export function Icon({ name, size = 24 }: { name: Name; size?: number }) {
  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.75"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
    >
      <path d={paths[name]} />
    </svg>
  );
}
