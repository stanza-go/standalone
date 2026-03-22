import { useState, useCallback } from "react";

export interface ColumnDef {
  key: string;
  label: string;
  defaultVisible?: boolean;
}

export function useColumnVisibility(pageKey: string, columns: ColumnDef[]) {
  const storageKey = `stanza-columns-${pageKey}`;

  const [visibility, setVisibility] = useState<Record<string, boolean>>(() => {
    try {
      const stored = localStorage.getItem(storageKey);
      if (stored) {
        const parsed = JSON.parse(stored);
        // Merge with defaults for any new columns added since last visit.
        const merged: Record<string, boolean> = {};
        for (const col of columns) {
          merged[col.key] = col.key in parsed ? parsed[col.key] : col.defaultVisible !== false;
        }
        return merged;
      }
    } catch {
      // Ignore parse errors.
    }
    const defaults: Record<string, boolean> = {};
    for (const col of columns) {
      defaults[col.key] = col.defaultVisible !== false;
    }
    return defaults;
  });

  const toggle = useCallback(
    (key: string) => {
      setVisibility((prev) => {
        const next = { ...prev, [key]: !prev[key] };
        localStorage.setItem(storageKey, JSON.stringify(next));
        return next;
      });
    },
    [storageKey],
  );

  const isVisible = useCallback(
    (key: string): boolean => {
      return visibility[key] !== false;
    },
    [visibility],
  );

  const visibleCount = columns.filter((c) => visibility[c.key] !== false).length;

  return { isVisible, toggle, visibleCount, columns };
}
