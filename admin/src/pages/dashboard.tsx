import { useAuth } from "@/lib/auth";
import { Button } from "@/components/ui/button";
import { LogOut } from "lucide-react";

export default function DashboardPage() {
  const { admin, logout } = useAuth();

  return (
    <div className="min-h-screen">
      <header className="border-b border-border">
        <div className="flex h-14 items-center justify-between px-6">
          <span className="text-sm font-semibold">Stanza Admin</span>
          <div className="flex items-center gap-3">
            <span className="text-sm text-muted-foreground">
              {admin?.email}
            </span>
            <Button variant="ghost" size="icon" onClick={logout}>
              <LogOut className="h-4 w-4" />
            </Button>
          </div>
        </div>
      </header>
      <main className="p-6">
        <h1 className="text-2xl font-semibold tracking-tight">Dashboard</h1>
        <p className="mt-1 text-sm text-muted-foreground">
          Welcome back, {admin?.name || admin?.email}.
        </p>
      </main>
    </div>
  );
}
