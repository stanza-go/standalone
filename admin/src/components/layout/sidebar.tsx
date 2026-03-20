import { NavLink, Outlet } from "react-router";
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
  KeyRound,
} from "lucide-react";
import { useState, type ReactNode } from "react";
import { cn } from "@/lib/utils";

interface NavItem {
  label: string;
  to: string;
  icon: ReactNode;
}

const navItems: NavItem[] = [
  { label: "Dashboard", to: "/", icon: <LayoutDashboard className="h-4 w-4" /> },
  { label: "Admin Users", to: "/admins", icon: <Users className="h-4 w-4" /> },
  { label: "Sessions", to: "/sessions", icon: <KeyRound className="h-4 w-4" /> },
  { label: "Cron Jobs", to: "/cron", icon: <Clock className="h-4 w-4" /> },
  { label: "Job Queue", to: "/queue", icon: <Inbox className="h-4 w-4" /> },
  { label: "Logs", to: "/logs", icon: <FileText className="h-4 w-4" /> },
  { label: "Database", to: "/database", icon: <Database className="h-4 w-4" /> },
  { label: "Settings", to: "/settings", icon: <Settings className="h-4 w-4" /> },
];

export default function SidebarLayout() {
  const { admin, logout } = useAuth();
  const [collapsed, setCollapsed] = useState(false);

  return (
    <div className="flex h-screen">
      <aside
        className={cn(
          "flex flex-col border-r border-border bg-muted/30 transition-all duration-200",
          collapsed ? "w-16" : "w-56"
        )}
      >
        <div className="flex h-14 items-center justify-between border-b border-border px-4">
          {!collapsed && (
            <span className="text-sm font-semibold tracking-tight">Stanza</span>
          )}
          <Button
            variant="ghost"
            size="icon"
            className="h-7 w-7"
            onClick={() => setCollapsed(!collapsed)}
          >
            {collapsed ? (
              <ChevronRight className="h-4 w-4" />
            ) : (
              <ChevronLeft className="h-4 w-4" />
            )}
          </Button>
        </div>

        <nav className="flex-1 space-y-1 px-2 py-3">
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
              {!collapsed && <span>{item.label}</span>}
            </NavLink>
          ))}
        </nav>

        <div className="border-t border-border p-3">
          {!collapsed ? (
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
