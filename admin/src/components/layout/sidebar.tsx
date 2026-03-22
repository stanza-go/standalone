import { NavLink, Outlet, useLocation, useNavigate } from "react-router";
import { useAuth } from "@/lib/auth";
import { Button } from "@/components/ui/button";
import { NotificationBell } from "@/components/notification-bell";
import { ThemeToggle } from "@/components/ui/theme-toggle";
import { CommandPalette } from "@/components/ui/command-palette";
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
  Bell,
  Upload,
  Webhook,
  Menu,
  X,
  Search,
} from "lucide-react";
import { Suspense, useState, useEffect, useCallback, type ReactNode } from "react";
import { Spinner } from "@/components/ui/spinner";
import { cn } from "@/lib/utils";

interface NavItem {
  label: string;
  to: string;
  icon: ReactNode;
}

interface NavSection {
  label?: string;
  items: NavItem[];
}

const navSections: NavSection[] = [
  {
    items: [
      { label: "Dashboard", to: "/", icon: <LayoutDashboard className="h-4 w-4" /> },
    ],
  },
  {
    label: "Users & Access",
    items: [
      { label: "Users", to: "/users", icon: <UsersRound className="h-4 w-4" /> },
      { label: "Admin Users", to: "/admins", icon: <Users className="h-4 w-4" /> },
      { label: "Sessions", to: "/sessions", icon: <KeyRound className="h-4 w-4" /> },
      { label: "API Keys", to: "/api-keys", icon: <KeySquare className="h-4 w-4" /> },
      { label: "Roles", to: "/roles", icon: <Shield className="h-4 w-4" /> },
    ],
  },
  {
    label: "System",
    items: [
      { label: "Cron Jobs", to: "/cron", icon: <Clock className="h-4 w-4" /> },
      { label: "Job Queue", to: "/queue", icon: <Inbox className="h-4 w-4" /> },
      { label: "Logs", to: "/logs", icon: <FileText className="h-4 w-4" /> },
      { label: "Database", to: "/database", icon: <Database className="h-4 w-4" /> },
      { label: "Webhooks", to: "/webhooks", icon: <Webhook className="h-4 w-4" /> },
    ],
  },
  {
    label: "Content",
    items: [
      { label: "Uploads", to: "/uploads", icon: <Upload className="h-4 w-4" /> },
      { label: "Notifications", to: "/notifications", icon: <Bell className="h-4 w-4" /> },
      { label: "Audit Log", to: "/audit", icon: <ScrollText className="h-4 w-4" /> },
    ],
  },
  {
    label: "Config",
    items: [
      { label: "Settings", to: "/settings", icon: <Settings className="h-4 w-4" /> },
    ],
  },
];

export default function SidebarLayout() {
  const { admin, logout } = useAuth();
  const navigate = useNavigate();
  const [collapsed, setCollapsed] = useState(false);
  const [mobileOpen, setMobileOpen] = useState(false);
  const [paletteOpen, setPaletteOpen] = useState(false);
  const location = useLocation();

  // Close mobile sidebar on route change.
  useEffect(() => {
    setMobileOpen(false);
  }, [location.pathname]);

  // Global Ctrl+K / Cmd+K to open command palette.
  const handleGlobalKeyDown = useCallback((e: KeyboardEvent) => {
    if ((e.metaKey || e.ctrlKey) && e.key === "k") {
      e.preventDefault();
      setPaletteOpen((prev) => !prev);
    }
  }, []);

  useEffect(() => {
    document.addEventListener("keydown", handleGlobalKeyDown);
    return () => document.removeEventListener("keydown", handleGlobalKeyDown);
  }, [handleGlobalKeyDown]);

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
        <div className="flex items-center gap-1">
          <Button
            variant="ghost"
            size="icon"
            className="h-8 w-8"
            onClick={() => setPaletteOpen(true)}
          >
            <Search className="h-4 w-4" />
          </Button>
          <ThemeToggle collapsed />
          <NotificationBell />
          <Button variant="ghost" size="icon" className="h-8 w-8" onClick={logout}>
            <LogOut className="h-4 w-4" />
          </Button>
        </div>
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
          "flex min-h-0 flex-col border-r border-border transition-all duration-200",
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
          <div className="flex items-center gap-0.5">
            {/* Desktop: notification bell */}
            <div className="hidden md:block">
              <NotificationBell collapsed={collapsed} />
            </div>
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
          </div>
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
          {/* Search trigger */}
          <button
            onClick={() => setPaletteOpen(true)}
            className="flex w-full items-center gap-3 rounded-md px-3 py-2 text-sm text-muted-foreground hover:bg-accent hover:text-accent-foreground transition-colors mb-1"
          >
            <Search className="h-4 w-4" />
            {(mobileOpen || !collapsed) && (
              <>
                <span className="flex-1 text-left">Search...</span>
                <kbd className="hidden sm:inline-flex h-5 items-center rounded border border-border bg-muted px-1.5 text-[10px] font-medium text-muted-foreground">
                  {navigator.platform.includes("Mac") ? "\u2318" : "Ctrl"}K
                </kbd>
              </>
            )}
          </button>
          {navSections.map((section, sectionIdx) => (
            <div key={section.label ?? sectionIdx} className={cn(sectionIdx > 0 && "mt-3")}>
              {section.label && (mobileOpen || !collapsed) && (
                <div className="px-3 pb-1 text-[11px] font-semibold uppercase tracking-wider text-muted-foreground/60">
                  {section.label}
                </div>
              )}
              {section.label && !mobileOpen && collapsed && (
                <div className="mx-auto mb-1 h-px w-6 bg-border" />
              )}
              {section.items.map((item) => (
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
            </div>
          ))}
        </nav>

        <div className="border-t border-border p-3">
          {(mobileOpen || !collapsed) ? (
            <div className="space-y-2">
              <div className="flex items-center justify-between">
                <button
                  className="min-w-0 text-left hover:opacity-80 transition-opacity"
                  onClick={() => navigate("/profile")}
                >
                  <p className="truncate text-sm font-medium">
                    {admin?.name || admin?.email}
                  </p>
                  <p className="truncate text-xs text-muted-foreground">
                    {admin?.role}
                  </p>
                </button>
                <Button variant="ghost" size="icon" className="h-7 w-7" onClick={logout}>
                  <LogOut className="h-4 w-4" />
                </Button>
              </div>
              <ThemeToggle />
            </div>
          ) : (
            <div className="flex flex-col items-center gap-2">
              <ThemeToggle collapsed />
              <Button
                variant="ghost"
                size="icon"
                className="h-7 w-7"
                onClick={logout}
              >
                <LogOut className="h-4 w-4" />
              </Button>
            </div>
          )}
        </div>
      </aside>

      <main className="flex-1 overflow-auto">
        <Suspense fallback={<div className="flex items-center justify-center h-full"><Spinner /></div>}>
          <Outlet />
        </Suspense>
      </main>

      <CommandPalette open={paletteOpen} onClose={() => setPaletteOpen(false)} />
    </div>
  );
}
