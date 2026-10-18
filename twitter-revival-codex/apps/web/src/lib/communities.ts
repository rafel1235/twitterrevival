"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { api, ApiError } from "./api";
import type { Author } from "./posts";

export type Community = {
  id: string;
  owner_id: string;
  name: string;
  description: string;
  rules: string;
  created_at: string;
  updated_at: string;
  members_count: number;
  is_member: boolean;
  is_owner: boolean;
  can_manage: boolean;
};
export type CommunityValues = Pick<Community, "name" | "description" | "rules">;
export type CommunityMember = Author;
export type CommunityPage<T> = { items: T[]; next_cursor?: string };

export function communitiesChanged() {
  window.dispatchEvent(new Event("radura:communities-changed"));
}

export function useCommunityPage<T extends { id: string }>(path: string) {
  const [data, setData] = useState<CommunityPage<T>>({ items: [] });
  const [pending, setPending] = useState(true);
  const [error, setError] = useState("");
  const active = useRef<AbortController | null>(null);
  const sequence = useRef(0);
  const lastCursor = useRef("");
  const load = useCallback(
    (cursor = "") => {
      active.current?.abort();
      const controller = new AbortController();
      active.current = controller;
      const request = ++sequence.current;
      lastCursor.current = cursor;
      return api<{ data: CommunityPage<T> }>(
        `${path}${path.includes("?") ? "&" : "?"}limit=20${cursor ? `&cursor=${encodeURIComponent(cursor)}` : ""}`,
        { signal: controller.signal },
      )
        .then((result) => {
          if (controller.signal.aborted || sequence.current !== request) return;
          setData((previous) => ({
            ...result.data,
            items: cursor
              ? [
                  ...previous.items,
                  ...result.data.items.filter(
                    (item) => !previous.items.some((old) => old.id === item.id),
                  ),
                ]
              : result.data.items,
          }));
          setError("");
        })
        .catch((err: unknown) => {
          if (controller.signal.aborted || sequence.current !== request) return;
          if (err instanceof ApiError && [401, 403, 404].includes(err.status))
            setData({ items: [] });
          setError(
            err instanceof Error ? err.message : "Caricamento non riuscito.",
          );
        })
        .finally(() => {
          if (!controller.signal.aborted && sequence.current === request)
            setPending(false);
        });
    },
    [path],
  );
  const refresh = useCallback(
    (cursor = "") => {
      setPending(true);
      return load(cursor);
    },
    [load],
  );
  useEffect(() => {
    void load();
    const update = () => void refresh();
    window.addEventListener("radura:communities-changed", update);
    return () => {
      active.current?.abort();
      window.removeEventListener("radura:communities-changed", update);
    };
  }, [load, refresh]);
  return {
    data,
    pending,
    error,
    load: refresh,
    retry: () => void refresh(lastCursor.current),
  };
}
