"use client";
import { useEffect, useState } from "react";
import { api } from "./api";
import type { Author } from "./posts";
export type Notification = {
  id: string;
  type: "follow" | "like" | "repost" | "reply" | "quote";
  actor: Author;
  post_id?: string;
  created_at: string;
  read_at: string | null;
};
export type NotificationPage = {
  items: Notification[];
  next_cursor?: string;
  unread_count: number;
};
export function notificationsChanged() {
  window.dispatchEvent(new Event("radura:notifications-changed"));
}
export function useUnreadNotifications(userID?: string) {
  const [state, setState] = useState<{ owner: string; count: number } | null>(
    null,
  );
  useEffect(() => {
    if (!userID) return;
    let stopped = false;
    let controller: AbortController | undefined;
    let timer: ReturnType<typeof setTimeout>;
    function load() {
      clearTimeout(timer);
      controller?.abort();
      const current = new AbortController();
      controller = current;
      if (document.visibilityState !== "visible") {
        timer = setTimeout(load, 30000);
        return;
      }
      void api<{ data: { unread_count: number } }>(
        "/notifications/unread-count",
        { signal: current.signal },
      )
        .then((result) => {
          if (!stopped && !current.signal.aborted)
            setState({ owner: userID!, count: result.data.unread_count });
        })
        .catch(() => {})
        .finally(() => {
          if (!stopped && !current.signal.aborted)
            timer = setTimeout(load, 30000);
        });
    }
    load();
    window.addEventListener("radura:notifications-changed", load);
    document.addEventListener("visibilitychange", load);
    return () => {
      stopped = true;
      clearTimeout(timer);
      controller?.abort();
      window.removeEventListener("radura:notifications-changed", load);
      document.removeEventListener("visibilitychange", load);
    };
  }, [userID]);
  return state && state.owner === userID ? state.count : 0;
}
