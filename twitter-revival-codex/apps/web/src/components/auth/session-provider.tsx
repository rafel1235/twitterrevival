"use client";

import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useState,
} from "react";
import { api, ApiError, type User } from "@/lib/api";

type Session = {
  user: User | null;
  loading: boolean;
  error: string;
  setUser: (user: User | null) => void;
  refresh: () => Promise<void>;
  logout: () => Promise<void>;
};
const SessionContext = createContext<Session | null>(null);

export function SessionProvider({ children }: { children: React.ReactNode }) {
  const [user, setUser] = useState<User | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const loadSession = useCallback((signal?: AbortSignal) => {
    return api<{ data: { user: User } }>("/auth/me", { signal })
      .then((result) => {
        setUser(result.data.user);
        setError("");
      })
      .catch((err: unknown) => {
        if (signal?.aborted) return;
        if (err instanceof ApiError && err.status === 401) {
          setUser(null);
          setError("");
        } else
          setError(
            "Impossibile verificare la sessione. Il servizio potrebbe essere offline.",
          );
      })
      .finally(() => {
        if (!signal?.aborted) setLoading(false);
      });
  }, []);
  const refresh = useCallback(async () => {
    setLoading(true);
    await loadSession();
  }, [loadSession]);
  useEffect(() => {
    const controller = new AbortController();
    void loadSession(controller.signal);
    return () => controller.abort();
  }, [loadSession]);
  async function logout() {
    await api<void>("/auth/logout", { body: {} });
    setUser(null);
    setError("");
  }
  return (
    <SessionContext.Provider
      value={{
        user,
        loading,
        error,
        setUser: (value) => {
          setUser(value);
          setError("");
        },
        refresh,
        logout,
      }}
    >
      {children}
    </SessionContext.Provider>
  );
}

export function useSession() {
  const session = useContext(SessionContext);
  if (!session) throw new Error("SessionProvider is missing");
  return session;
}
