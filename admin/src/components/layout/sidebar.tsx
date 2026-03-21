import { NavLink, Outlet, useLocation } from "react-router";
import { useAuth } from "@/lib/auth";
import { Button } from "@/components/ui/button";
import {
  LayoutDashboard,
  LogOut,
  Settings,
  Database,
  FileText,
  Clock,
  Inbox,
  ChevronLeft,
  ChevronRight,
  Users,
  UsersRound,
  KeyRound,
  KeySquare,
  ScrollText,
  Shield,
  Menu,
  X,
} from "lucide-react";
import { useState, useEffect, type ReactNode } from "react";
import { cn } from "@/lib/utils";

interface NavItem {
  label: string;
  to: string;
  icon: ReactNode;
}

const navItems: NavItem[] = [
  { label: "Dashboard", to: "/", icon: <LayoutDashboard className="h-4 w-4" /> },
  { label: "Users", to: "/users", icon: <UsersRound className="h-4 w-4" /> },
  { label: "Admin Users", to: "/admins", icon: <Users className="h-4 w-4" /> },
  { label: "Sessions", to: "/sessions", icon: <KeyRound className="h-4 w-4" /> },
  { label: "API Keys", to: "/api-keys", icon: <KeySquare className="h-4 w-4" /> },
  { label: "Cron Jobs", to: "/cron", icon: <Clock className="h-4 w-4" /> },
  { label: "Job Queue", to: "/queue", icon: <Inbox className="h-4 w-4" /> },
  { label: "Logs", to: "/logs", icon: <FileText className="h-4 w-4" /> },
  { label: "Database", to: "/database", icon: <Database className="h-4 w-4" /> },
  { label: "Audit Log", to: "/audit", icon: <ScrollText className="h-4 w-4" /> },
  { label: "Roles", to: "/roles", icon: <Shield className="h-4 w-4" /> },
  { label: "Settings", to: "/settings", icon: <Settings className="h-4 w-4" /> },
];

export default function SidebarLayout() {
  const { admin, logout } = useAuth();
  const [collapsed, setCollapsed] = useState(false);
  const [mobileOpen, setMobileOpen] = useState(false);
  const location = useLocation();

  // Close mobile sidebar on route change.
  useEffect(() => {
    setMobileOpen(false);
  }, [location.pathname]);

  return (
    <div className="flex h-screen flex-col md:flex-row">
      {/* Mobile top bar */}
      <header className="flex h-14 shrink-0 items-center justify-between border-b border-border px-4 md:hidden">
        <div className="flex items-center gap-3">
          <Button
            variant="ghost"
            size="icon"
            className="h-8 w-8"
            onClick={() => setMobileOpen(true)}
          >
            <Menu className="h-5 w-5" />
          </Button>
          <span className="text-sm font-semibold tracking-tight">Stanza</span>
        </div>
        <Button variant="ghost" size="icon" className="h-8 w-8" onClick={logout}>
          <LogOut className="h-4 w-4" />
        </Button>
      </header>

      {/* Mobile backdrop */}
      {mobileOpen && (
        <div
          className="fixed inset-0 z-40 bg-black/50 md:hidden"
          onClick={() => setMobileOpen(false)}
        />
      )}

      {/* Sidebar */}
      <aside
        className={cn(
          "flex flex-col border-r border-border transition-all duration-200",
          // Mobile: fixed overlay that slides in/out
          "fixed inset-y-0 left-0 z-50 w-64 bg-background shadow-xl",
          mobileOpen ? "translate-x-0" : "-translate-x-full",
          // Desktop: static sidebar in flex layout
          "md:relative md:z-auto md:translate-x-0 md:bg-muted/30 md:shadow-none",
          collapsed ? "md:w-16" : "md:w-56"
        )}
      >
        <div className="flex h-14 items-center justify-between border-b border-border px-4">
          {(mobileOpen || !collapsed) && (
            <span className="text-sm font-semibold tracking-tight">Stanza</span>
          )}
          {/* Desktop: collapse toggle */}
          <Button
            variant="ghost"
            size="icon"
            className="hidden h-7 w-7 md:inline-flex"
            onClick={() => setCollapsed(!collapsed)}
          >
            {collapsed ? (
              <ChevronRight className="h-4 w-4" />
            ) : (
              <ChevronLeft className="h-4 w-4" />
            )}
          </Button>
          {/* Mobile: close button */}
          <Button
            variant="ghost"
            size="icon"
            className="h-7 w-7 md:hidden"
            onClick={() => setMobileOpen(false)}
          >
            <X className="h-4 w-4" />
          </Button>
        </div>

        <nav className="flex-1 space-y-1 overflow-y-auto px-2 py-3">
          {navItems.map((item) => (
            <NavLink
              key={item.to}
              to={item.to}
              end={item.to === "/"}
              className={({ isActive }) =>
                cn(
                  "flex items-center gap-3 rounded-md px-3 py-2 text-sm transition-colors",
                  isActive
                    ? "bg-primary text-primary-foreground"
                    : "text-muted-foreground hover:bg-accent hover:text-accent-foreground"
                )
              }
            >
              {item.icon}
              {(mobileOpen || !collapsed) && <span>{item.label}</span>}
            </NavLink>
          ))}
        </nav>

        <div className="border-t border-border p-3">
          {(mobileOpen || !collapsed) ? (
            <div className="flex items-center justify-between">
              <div className="min-w-0">
                <p className="truncate text-sm font-medium">
                  {admin?.name || admin?.email}
                </p>
                <p className="truncate text-xs text-muted-foreground">
                  {admin?.role}
                </p>
              </div>
              <Button variant="ghost" size="icon" className="h-7 w-7" onClick={logout}>
                <LogOut className="h-4 w-4" />
              </Button>
            </div>
          ) : (
            <Button
              variant="ghost"
              size="icon"
              className="mx-auto block h-7 w-7"
              onClick={logout}
            >
              <LogOut className="h-4 w-4" />
            </Button>
          )}
        </div>
      </aside>

      <main className="flex-1 overflow-auto">
        <Outlet />
      </main>
    </div>
  );
}
