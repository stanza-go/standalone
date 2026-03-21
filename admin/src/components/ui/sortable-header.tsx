import { ArrowUp, ArrowDown, ArrowUpDown } from "lucide-react";

export type SortDirection = "asc" | "desc";

export interface SortState {
  column: string;
  direction: SortDirection;
}

interface SortableHeaderProps {
  label: string;
  column: string;
  sort: SortState;
  onSort: (column: string) => void;
  className?: string;
}

// SortableHeader renders a clickable table header that toggles sort direction.
// Clicking the active column toggles asc/desc. Clicking a different column
// activates it with desc as the default direction.
export function SortableHeader({ label, column, sort, onSort, className = "" }: SortableHeaderProps) {
  const active = sort.column === column;

  return (
    <th
      className={`text-left p-3 font-medium cursor-pointer select-none hover:bg-muted/80 transition-colors ${className}`}
      onClick={() => onSort(column)}
    >
      <span className="inline-flex items-center gap-1">
        {label}
        {active ? (
          sort.direction === "asc" ? (
            <ArrowUp className="h-3.5 w-3.5 text-foreground" />
          ) : (
            <ArrowDown className="h-3.5 w-3.5 text-foreground" />
          )
        ) : (
          <ArrowUpDown className="h-3.5 w-3.5 text-muted-foreground/50" />
        )}
      </span>
    </th>
  );
}

// useSort manages sort state and provides a toggle handler.
// When a column is clicked:
// - If it's the current column, toggle direction
// - If it's a new column, set it as active with defaultDir
export function useSort(defaultColumn: string, defaultDirection: SortDirection = "desc"): [SortState, (column: string) => void] {
  const [sort, setSort] = useState<SortState>({ column: defaultColumn, direction: defaultDirection });

  const toggleSort = useCallback((column: string) => {
    setSort((prev) => {
      if (prev.column === column) {
        return { column, direction: prev.direction === "asc" ? "desc" : "asc" };
      }
      return { column, direction: defaultDirection };
    });
  }, [defaultDirection]);

  return [sort, toggleSort];
}

// Re-export hooks so pages don't need a separate import
import { useState, useCallback } from "react";
