import { createContext, useContext, useEffect, useRef, useState } from "react";
import { authApi, type AuthResponse } from "../api/auth";
import { setAccessToken } from "../api/client";

interface AuthUser {
  userId: string;
  orgId: string;
  role: string;
  email: string;
}

interface AuthContextValue {
  user: AuthUser | null;
  loading: boolean;
  login: (email: string, password: string, orgSlug: string) => Promise<void>;
  signup: (payload: Parameters<typeof authApi.signup>[0]) => Promise<void>;
  logout: () => void;
}

const AuthContext = createContext<AuthContextValue | null>(null);

const REFRESH_INTERVAL_MS = 13 * 60 * 1000;

function applyAuth(data: AuthResponse): AuthUser {
  setAccessToken(data.access_token);
  return {
    userId: data.user_id,
    orgId: data.org_id,
    role: data.role,
    email: "",
  };
}

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const [user, setUser] = useState<AuthUser | null>(null);
  const [loading, setLoading] = useState(true);
  const timerRef = useRef<ReturnType<typeof setInterval> | null>(null);

  function startRefreshTimer() {
    if (timerRef.current) clearInterval(timerRef.current);
    timerRef.current = setInterval(async () => {
      try {
        const res = await authApi.refresh();
        setUser(applyAuth(res.data));
      } catch {
        setUser(null);
        setAccessToken(null);
      }
    }, REFRESH_INTERVAL_MS);
  }

  useEffect(() => {
    authApi
      .refresh()
      .then((res) => {
        const u = applyAuth(res.data);
        authApi
          .me()
          .then((me) => setUser({ ...u, email: me.data.email }))
          .catch(() => setUser(u));
        startRefreshTimer();
      })
      .catch(() => setUser(null))
      .finally(() => setLoading(false));

    return () => {
      if (timerRef.current) clearInterval(timerRef.current);
    };
  }, []);

  async function login(email: string, password: string, orgSlug: string) {
    const res = await authApi.login(email, password, orgSlug);
    const u = applyAuth(res.data);
    setUser({ ...u, email }); // optimistic — use provided email until /me confirms
    startRefreshTimer();
    // Fetch the canonical email from the server to keep state authoritative.
    authApi
      .me()
      .then((me) =>
        setUser((prev) => (prev ? { ...prev, email: me.data.email } : prev)),
      )
      .catch(() => undefined);
  }

  async function signup(payload: Parameters<typeof authApi.signup>[0]) {
    const res = await authApi.signup(payload);
    const u = applyAuth(res.data);
    setUser({ ...u, email: payload.email }); // optimistic
    startRefreshTimer();
    // Fetch canonical email from server.
    authApi
      .me()
      .then((me) =>
        setUser((prev) => (prev ? { ...prev, email: me.data.email } : prev)),
      )
      .catch(() => undefined);
  }

  function logout() {
    // Revoke the server-side refresh token (best-effort, fire-and-forget).
    authApi.logout().catch(() => undefined);
    setAccessToken(null);
    setUser(null);
    if (timerRef.current) clearInterval(timerRef.current);
  }

  return (
    <AuthContext.Provider value={{ user, loading, login, signup, logout }}>
      {children}
    </AuthContext.Provider>
  );
}

export function useAuth(): AuthContextValue {
  const ctx = useContext(AuthContext);
  if (!ctx) throw new Error("useAuth must be used inside AuthProvider");
  return ctx;
}
