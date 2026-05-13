import { useEffect } from "react";

type SaveFn = () => Promise<void>;

// Debounces a save function. Re-runs whenever deps change.
// fn identity changes do NOT re-trigger the timer — only listed deps do.
export function useAutoSave(fn: SaveFn, deps: unknown[], delayMs = 1200) {
  useEffect(() => {
    const id = setTimeout(() => {
      fn().catch(() => {
        // auto-save failures are non-fatal
      });
    }, delayMs);
    return () => clearTimeout(id);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, deps);
}
