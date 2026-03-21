import { Loader2 } from "lucide-react";
import { cn } from "@/lib/utils";

function Spinner({ className }: { className?: string }) {
  return (
    <div className={cn("flex items-center gap-2 text-sm text-muted-foreground", className)}>
      <Loader2 className="h-4 w-4 animate-spin" />
      <span>Loading...</span>
    </div>
  );
}

export { Spinner };
