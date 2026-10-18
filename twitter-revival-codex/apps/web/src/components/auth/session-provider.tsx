"use client";

import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useRef,
  useState,
} from "react";
import { api, ApiError, type User } from "@/lib/api";

type Session = {
  user: User | null;
  loading: boolean;
  error: string;
  expired: boolean;
  setUser: (user: User | null) => void;
  refresh: () => Promise<void>;
  logout: () => Promise<void>;
};
const SessionContext = createContext<Session | null>(null);

export function SessionProvider({ children }: { children: React.ReactNode }) {
  const [user, setUser] = useState<User | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [expired, setExpired] = useState(false);
  const authenticated = useRef(false);
  const generation = useRef(0);
  const loadSession = useCallback((signal?: AbortSignal) => {
    const request = ++generation.current;
    return api<{ data: { user: User } }>("/auth/me", { signal })
      .then((result) => {
        if (signal?.aborted || request !== generation.current) return;
        setUser(result.data.user);
        authenticated.current = true;
        setExpired(false);
        setError("");
      })
      .catch((err: unknown) => {
        if (signal?.aborted || request !== generation.current) return;
        if (err instanceof ApiError && err.status === 401) {
          setUser(null);
          setError("");
        } else
          setError(
            "Impossibile verificare la sessione. Il servizio potrebbe essere offline.",
          );
      })
      .finally(() => {
        if (!signal?.aborted && request === generation.current)
          setLoading(false);
      });
  }, []);
  const refresh = useCallback(async () => {
    setLoading(true);
    await loadSession();
  }, [loadSession]);
  useEffect(() => {
    const controller = new AbortController();
    const expire = () => {
      if (authenticated.current) setExpired(true);
      authenticated.current = false;
      generation.current++;
      setUser(null);
      setError("");
      setLoading(false);
    };
    window.addEventListener("radura:session-expired", expire);
    void loadSession(controller.signal);
    return () => {
      controller.abort();
      window.removeEventListener("radura:session-expired", expire);
    };
  }, [loadSession]);
  async function logout() {
    await api<void>("/auth/logout", { body: {} });
    generation.current++;
    setUser(null);
    authenticated.current = false;
    setExpired(false);
    setError("");
    setLoading(false);
  }
  return (
    <SessionContext.Provider
      value={{
        user,
        loading,
        error,
        expired,
        setUser: (value) => {
          generation.current++;
          setUser(value);
          authenticated.current = !!value;
          setExpired(false);
          setError("");
          setLoading(false);
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
