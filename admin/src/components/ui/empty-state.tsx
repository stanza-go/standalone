import { Inbox } from "lucide-react";
import { cn } from "@/lib/utils";

function EmptyState({
  message,
  icon,
  className,
}: {
  message: string;
  icon?: React.ReactNode;
  className?: string;
}) {
  return (
    <div className={cn("flex flex-col items-center justify-center gap-2 py-8 text-muted-foreground", className)}>
      {icon || <Inbox className="h-8 w-8 opacity-40" />}
      <p className="text-sm">{message}</p>
    </div>
  );
}

function TableEmptyRow({
  colSpan,
  message,
}: {
  colSpan: number;
  message: string;
}) {
  return (
    <tr>
      <td colSpan={colSpan}>
        <EmptyState message={message} className="py-12" />
      </td>
    </tr>
  );
}

export { EmptyState, TableEmptyRow };
