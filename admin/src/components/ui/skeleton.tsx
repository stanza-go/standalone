import { cn } from "@/lib/utils";

function Skeleton({ className }: { className?: string }) {
  return (
    <div
      className={cn(
        "animate-pulse rounded bg-muted",
        className,
      )}
    />
  );
}

function TableSkeleton({
  columns,
  rows = 5,
}: {
  columns: { width?: string; hidden?: string }[];
  rows?: number;
}) {
  return (
    <div className="overflow-x-auto">
      <table className="w-full text-sm">
        <thead>
          <tr className="bg-muted/50 border-b">
            {columns.map((col, i) => (
              <th
                key={i}
                className={cn("p-3", col.hidden)}
              >
                <Skeleton className={cn("h-4", col.width || "w-20")} />
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {Array.from({ length: rows }, (_, rowIdx) => (
            <tr key={rowIdx} className="border-b border-border">
              {columns.map((col, colIdx) => (
                <td
                  key={colIdx}
                  className={cn("p-3", col.hidden)}
                >
                  <Skeleton
                    className={cn(
                      "h-4",
                      col.width || "w-20",
                      rowIdx % 2 === 0 ? "" : "opacity-75",
                    )}
                  />
                </td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

export { Skeleton, TableSkeleton };
