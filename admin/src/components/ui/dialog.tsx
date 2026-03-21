import { useEffect, useRef, type HTMLAttributes, type ReactNode } from "react";
import { cn } from "@/lib/utils";
import { X } from "lucide-react";
import { Button } from "./button";

interface DialogProps {
  open: boolean;
  onClose: () => void;
  children: ReactNode;
  className?: string;
}

function Dialog({ open, onClose, children, className }: DialogProps) {
  const ref = useRef<HTMLDialogElement>(null);

  useEffect(() => {
    const dialog = ref.current;
    if (!dialog) return;

    if (open && !dialog.open) {
      dialog.showModal();
    } else if (!open && dialog.open) {
      dialog.close();
    }
  }, [open]);

  // Native <dialog> fires "close" on Escape — sync our state.
  function handleClose() {
    onClose();
  }

  // Close on backdrop click (click on <dialog> itself, not its children).
  function handleClick(e: React.MouseEvent<HTMLDialogElement>) {
    if (e.target === ref.current) {
      onClose();
    }
  }

  return (
    <dialog
      ref={ref}
      onClose={handleClose}
      onClick={handleClick}
      className={cn(
        "backdrop:bg-black/50 bg-transparent p-0 m-auto",
        "max-h-[85vh] overflow-visible",
        className
      )}
    >
      <div className="bg-card text-card-foreground rounded-lg shadow-lg w-full max-w-md">
        {children}
      </div>
    </dialog>
  );
}

function DialogHeader({ className, ...props }: HTMLAttributes<HTMLDivElement>) {
  return (
    <div
      className={cn("flex items-center justify-between p-4 border-b", className)}
      {...props}
    />
  );
}

function DialogTitle({ className, ...props }: HTMLAttributes<HTMLHeadingElement>) {
  return (
    <h2
      className={cn("text-lg font-semibold", className)}
      {...props}
    />
  );
}

function DialogCloseButton({ onClick }: { onClick: () => void }) {
  return (
    <Button variant="ghost" size="sm" onClick={onClick}>
      <X className="h-4 w-4" />
    </Button>
  );
}

function DialogBody({ className, ...props }: HTMLAttributes<HTMLDivElement>) {
  return (
    <div className={cn("p-4", className)} {...props} />
  );
}

function DialogFooter({ className, ...props }: HTMLAttributes<HTMLDivElement>) {
  return (
    <div
      className={cn("flex justify-end gap-2 p-4 pt-0", className)}
      {...props}
    />
  );
}

export { Dialog, DialogHeader, DialogTitle, DialogCloseButton, DialogBody, DialogFooter };
