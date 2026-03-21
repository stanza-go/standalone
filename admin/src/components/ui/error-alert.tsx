import { AlertTriangle, RefreshCw } from "lucide-react";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";

function ErrorAlert({
  message,
  onRetry,
  onDismiss,
  className,
}: {
  message: string;
  onRetry?: () => void;
  onDismiss?: () => void;
  className?: string;
}) {
  return (
    <div
      className={cn(
        "flex items-start gap-3 rounded-md border border-destructive/50 bg-destructive/5 p-3 text-sm text-destructive",
        className
      )}
    >
      <AlertTriangle className="mt-0.5 h-4 w-4 shrink-0" />
      <span className="flex-1">{message}</span>
      <div className="flex shrink-0 items-center gap-1">
        {onRetry && (
          <Button variant="ghost" size="sm" className="h-7 px-2 text-destructive hover:text-destructive" onClick={onRetry}>
            <RefreshCw className="mr-1 h-3 w-3" />
            Retry
          </Button>
        )}
        {onDismiss && (
          <button
            onClick={onDismiss}
            className="ml-1 rounded p-0.5 hover:bg-destructive/10"
            aria-label="Dismiss"
          >
            &times;
          </button>
        )}
      </div>
    </div>
  );
}

export { ErrorAlert };
