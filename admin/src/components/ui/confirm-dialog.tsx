import type { ReactNode } from "react";
import { Button } from "./button";
import {
  Dialog,
  DialogHeader,
  DialogTitle,
  DialogCloseButton,
  DialogBody,
  DialogFooter,
} from "./dialog";

interface ConfirmDialogProps {
  open: boolean;
  onClose: () => void;
  onConfirm: () => void;
  title: string;
  message: string;
  details?: ReactNode;
  confirmLabel?: string;
  variant?: "destructive" | "default";
  loading?: boolean;
}

function ConfirmDialog({
  open,
  onClose,
  onConfirm,
  title,
  message,
  details,
  confirmLabel = "Confirm",
  variant = "destructive",
  loading = false,
}: ConfirmDialogProps) {
  return (
    <Dialog open={open} onClose={onClose}>
      <DialogHeader>
        <DialogTitle>{title}</DialogTitle>
        <DialogCloseButton onClick={onClose} />
      </DialogHeader>

      <DialogBody>
        <p className="text-sm text-muted-foreground">{message}</p>
        {details && (
          <div className="mt-3 p-3 bg-muted rounded-md text-sm space-y-1">
            {details}
          </div>
        )}
      </DialogBody>

      <DialogFooter>
        <Button variant="outline" onClick={onClose} disabled={loading}>
          Cancel
        </Button>
        <Button
          variant={variant}
          onClick={onConfirm}
          disabled={loading}
        >
          {loading ? `${confirmLabel.replace(/\.\.\.$/, "")}...` : confirmLabel}
        </Button>
      </DialogFooter>
    </Dialog>
  );
}

export { ConfirmDialog };
export type { ConfirmDialogProps };
