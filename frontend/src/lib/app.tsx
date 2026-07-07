import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useRef,
  useState,
  type ReactNode,
} from "react";
import { Events } from "@wailsio/runtime";
import { TrackerService, type HomeState } from "../../bindings/timetrack";

interface AppContextValue {
  state: HomeState | null;
  /** Ticks every second; elapsed times are derived from timestamps against
   *  this on each render, never accumulated. */
  now: Date;
  error: string | null;
  clearError(): void;
  refresh(): Promise<void>;
  /** Runs a backend call, refreshing state on success and toasting errors. */
  run<T>(p: Promise<T>): Promise<T | undefined>;
}

const AppContext = createContext<AppContextValue | null>(null);

export function AppProvider({ children }: { children: ReactNode }) {
  const [state, setState] = useState<HomeState | null>(null);
  const [now, setNow] = useState(() => new Date());
  const [error, setError] = useState<string | null>(null);
  const errorTimer = useRef<ReturnType<typeof setTimeout>>(undefined);

  const showError = useCallback((e: unknown) => {
    setError(e instanceof Error ? e.message : String(e));
    clearTimeout(errorTimer.current);
    errorTimer.current = setTimeout(() => setError(null), 6000);
  }, []);

  const refresh = useCallback(async () => {
    try {
      setState(await TrackerService.HomeState());
    } catch (e) {
      showError(e);
    }
  }, [showError]);

  useEffect(() => {
    void refresh();
    // Backend emits this after every mutation, from any window.
    const off = Events.On("day:changed", () => void refresh());
    // Re-sync after sleep/resume or refocus, when timers were suspended.
    const onFocus = () => void refresh();
    window.addEventListener("focus", onFocus);
    const tick = setInterval(() => setNow(new Date()), 1000);
    return () => {
      off();
      window.removeEventListener("focus", onFocus);
      clearInterval(tick);
    };
  }, [refresh]);

  const run = useCallback(
    async <T,>(p: Promise<T>): Promise<T | undefined> => {
      try {
        const result = await p;
        await refresh();
        return result;
      } catch (e) {
        showError(e);
        return undefined;
      }
    },
    [refresh, showError],
  );

  return (
    <AppContext.Provider
      value={{ state, now, error, clearError: () => setError(null), refresh, run }}
    >
      {children}
    </AppContext.Provider>
  );
}

export function useApp(): AppContextValue {
  const ctx = useContext(AppContext);
  if (!ctx) throw new Error("useApp outside AppProvider");
  return ctx;
}
