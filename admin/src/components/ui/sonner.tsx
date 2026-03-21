import { Toaster as Sonner, type ToasterProps } from "sonner";

function Toaster({ ...props }: ToasterProps) {
  return (
    <Sonner
      className="toaster group"
      style={
        {
          "--normal-bg": "var(--popover)",
          "--normal-text": "var(--popover-foreground)",
          "--normal-border": "var(--border)",
          "--success-bg": "oklch(0.95 0.05 150)",
          "--success-text": "oklch(0.3 0.1 150)",
          "--success-border": "oklch(0.85 0.1 150)",
          "--error-bg": "oklch(0.95 0.05 27)",
          "--error-text": "oklch(0.4 0.15 27)",
          "--error-border": "oklch(0.85 0.1 27)",
        } as React.CSSProperties
      }
      toastOptions={{
        classNames: {
          toast:
            "group toast group-[.toaster]:bg-[var(--normal-bg)] group-[.toaster]:text-[var(--normal-text)] group-[.toaster]:border-[var(--normal-border)] group-[.toaster]:shadow-lg group-[.toaster]:rounded-lg",
          success:
            "group-[.toaster]:!bg-[var(--success-bg)] group-[.toaster]:!text-[var(--success-text)] group-[.toaster]:!border-[var(--success-border)]",
          error:
            "group-[.toaster]:!bg-[var(--error-bg)] group-[.toaster]:!text-[var(--error-text)] group-[.toaster]:!border-[var(--error-border)]",
          description: "group-[.toast]:text-muted-foreground",
        },
      }}
      {...props}
    />
  );
}

export { Toaster };
