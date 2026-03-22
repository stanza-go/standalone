import { useState } from "react";

export type SortDirection = "asc" | "desc";

export interface SortState {
  column: string;
  direction: SortDirection;
}

export function useSort(defaultColumn: string, defaultDirection: SortDirection = "desc"): [SortState, (column: string) => void] {
  const [sort, setSort] = useState<SortState>({ column: defaultColumn, direction: defaultDirection });

  function toggle(column: string) {
    setSort((prev) => {
      if (prev.column === column) {
        return { column, direction: prev.direction === "asc" ? "desc" : "asc" };
      }
      return { column, direction: defaultDirection };
    });
  }

  return [sort, toggle];
}
