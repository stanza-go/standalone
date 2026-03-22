import { useState, useRef, useEffect } from "react";
import { Button } from "@/components/ui/button";
import { Columns3 } from "lucide-react";

interface ColumnToggleProps {
  columns: { key: string; label: string }[];
  isVisible: (key: string) => boolean;
  toggle: (key: string) => void;
}

export function ColumnToggle({ columns, isVisible, toggle }: ColumnToggleProps) {
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!open) return;
    function handleClick(e: MouseEvent) {
      if (ref.current && !ref.current.contains(e.target as Node)) {
        setOpen(false);
      }
    }
    document.addEventListener("mousedown", handleClick);
    return () => document.removeEventListener("mousedown", handleClick);
  }, [open]);

  return (
    <div className="relative" ref={ref}>
      <Button
        variant="outline"
        size="sm"
        onClick={() => setOpen(!open)}
        title="Toggle columns"
      >
        <Columns3 className="h-4 w-4 mr-2" />
        Columns
      </Button>
      {open && (
        <div className="absolute right-0 top-full mt-1 z-50 w-48 rounded-md border bg-popover p-1 shadow-md">
          <p className="px-2 py-1.5 text-xs font-medium text-muted-foreground">
            Toggle columns
          </p>
          {columns.map((col) => (
            <label
              key={col.key}
              className="flex items-center gap-2 px-2 py-1.5 text-sm rounded-sm hover:bg-accent cursor-pointer"
            >
              <input
                type="checkbox"
                checked={isVisible(col.key)}
                onChange={() => toggle(col.key)}
                className="rounded border-input"
              />
              {col.label}
            </label>
          ))}
        </div>
      )}
    </div>
  );
}
