import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useRef,
  useState,
  type ReactNode,
} from "react";
import { get, post } from "@/lib/api";

export interface Admin {
  id: number;
  email: string;
  name: string;
  role: string;
}

interface AuthState {
  admin: Admin | null;
  loading: boolean;
}

interface AuthContextValue extends AuthState {
  login: (email: string, password: string) => Promise<void>;
  logout: () => Promise<void>;
}

const AuthContext = createContext<AuthContextValue | null>(null);

const STATUS_POLL_MS = 60_000;

export function AuthProvider({ children }: { children: ReactNode }) {
  const [state, setState] = useState<AuthState>({ admin: null, loading: true });
  const pollRef = useRef<ReturnType<typeof setInterval> | null>(null);

  const checkStatus = useCallback(async () => {
    try {
      const data = await get<{ admin: Admin }>("/admin/auth");
      setState({ admin: data.admin, loading: false });
    } catch {
      setState({ admin: null, loading: false });
    }
  }, []);

  const login = useCallback(async (email: string, password: string) => {
    const data = await post<{ admin: Admin }>("/admin/auth/login", {
      email,
      password,
    });
    setState({ admin: data.admin, loading: false });
  }, []);

  const logout = useCallback(async () => {
    try {
      await post("/admin/auth/logout");
    } catch {
      // Always clear local state even if the request fails
    }
    setState({ admin: null, loading: false });
  }, []);

  useEffect(() => {
    checkStatus();
  }, [checkStatus]);

  useEffect(() => {
    if (!state.admin) {
      if (pollRef.current) {
        clearInterval(pollRef.current);
        pollRef.current = null;
      }
      return;
    }

    pollRef.current = setInterval(() => {
      if (document.visibilityState === "visible") {
        checkStatus();
      }
    }, STATUS_POLL_MS);

    return () => {
      if (pollRef.current) {
        clearInterval(pollRef.current);
        pollRef.current = null;
      }
    };
  }, [state.admin, checkStatus]);

  return (
    <AuthContext value={{ ...state, login, logout }}>
      {children}
    </AuthContext>
  );
}

export function useAuth(): AuthContextValue {
  const ctx = useContext(AuthContext);
  if (!ctx) {
    throw new Error("useAuth must be used within an AuthProvider");
  }
  return ctx;
}
