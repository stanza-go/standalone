import type { ReactNode } from "react";
import { Button } from "./button";
import { X } from "lucide-react";

interface BulkActionBarProps {
  count: number;
  onClear: () => void;
  children: ReactNode;
}

export function BulkActionBar({ count, onClear, children }: BulkActionBarProps) {
  if (count === 0) return null;

  return (
    <div className="fixed bottom-6 left-1/2 -translate-x-1/2 z-50 flex items-center gap-3 px-4 py-2.5 bg-card border rounded-lg shadow-lg">
      <span className="text-sm font-medium whitespace-nowrap">
        {count} selected
      </span>
      <div className="h-4 w-px bg-border" />
      <div className="flex items-center gap-2">
        {children}
      </div>
      <Button variant="ghost" size="sm" onClick={onClear} title="Clear selection">
        <X className="h-4 w-4" />
      </Button>
    </div>
  );
}
