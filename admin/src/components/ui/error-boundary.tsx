import { Component, type ErrorInfo, type ReactNode } from "react";
import { AlertTriangle, RefreshCw, Home } from "lucide-react";
import { Button } from "@/components/ui/button";

interface Props {
  children: ReactNode;
}

interface State {
  hasError: boolean;
  error: Error | null;
}

class ErrorBoundary extends Component<Props, State> {
  constructor(props: Props) {
    super(props);
    this.state = { hasError: false, error: null };
  }

  static getDerivedStateFromError(error: Error): State {
    return { hasError: true, error };
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    console.error("ErrorBoundary caught:", error, info.componentStack);
  }

  render() {
    if (!this.state.hasError) {
      return this.props.children;
    }

    return (
      <div className="flex min-h-screen items-center justify-center bg-background p-4">
        <div className="w-full max-w-md space-y-6 text-center">
          <div className="mx-auto flex h-16 w-16 items-center justify-center rounded-full bg-destructive/10">
            <AlertTriangle className="h-8 w-8 text-destructive" />
          </div>
          <div className="space-y-2">
            <h1 className="text-2xl font-semibold text-foreground">Something went wrong</h1>
            <p className="text-sm text-muted-foreground">
              An unexpected error occurred. You can try reloading the page or going back to the dashboard.
            </p>
          </div>
          {this.state.error && (
            <div className="rounded-md bg-muted p-3 text-left">
              <p className="font-mono text-xs text-muted-foreground break-all">{this.state.error.message}</p>
            </div>
          )}
          <div className="flex items-center justify-center gap-3">
            <Button
              variant="outline"
              onClick={() => {
                const base = import.meta.env.BASE_URL.replace(/\/+$/, "") || "";
                window.location.href = base + "/";
              }}
            >
              <Home className="mr-2 h-4 w-4" />
              Dashboard
            </Button>
            <Button onClick={() => window.location.reload()}>
              <RefreshCw className="mr-2 h-4 w-4" />
              Reload Page
            </Button>
          </div>
        </div>
      </div>
    );
  }
}

export { ErrorBoundary };
