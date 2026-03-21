import { useEffect, useRef, useState, useCallback } from "react";
import { useNavigate } from "react-router";
import { useAuth } from "@/lib/auth";
import { useTheme } from "@/lib/theme";
import { cn } from "@/lib/utils";
import {
  LayoutDashboard,
  Users,
  UsersRound,
  KeyRound,
  KeySquare,
  Clock,
  Inbox,
  FileText,
  Database,
  Upload,
  ScrollText,
  Bell,
  Shield,
  Settings,
  User,
  LogOut,
  Sun,
  Moon,
  Monitor,
  Search,
} from "lucide-react";

interface CommandItem {
  id: string;
  label: string;
  icon: React.ReactNode;
  group: "users-access" | "system" | "content" | "config" | "other" | "actions";
  keywords?: string;
  onSelect: () => void;
}

interface CommandPaletteProps {
  open: boolean;
  onClose: () => void;
}

export function CommandPalette({ open, onClose }: CommandPaletteProps) {
  const navigate = useNavigate();
  const { logout } = useAuth();
  const { theme, setTheme } = useTheme();
  const [query, setQuery] = useState("");
  const [activeIndex, setActiveIndex] = useState(0);
  const inputRef = useRef<HTMLInputElement>(null);
  const dialogRef = useRef<HTMLDialogElement>(null);
  const listRef = useRef<HTMLDivElement>(null);

  const go = useCallback(
    (path: string) => {
      onClose();
      navigate(path);
    },
    [navigate, onClose],
  );

  const items: CommandItem[] = [
    // Dashboard & Profile
    { id: "dashboard", label: "Dashboard", icon: <LayoutDashboard className="h-4 w-4" />, group: "other", onSelect: () => go("/") },
    { id: "profile", label: "Profile", icon: <User className="h-4 w-4" />, group: "other", keywords: "account my profile password", onSelect: () => go("/profile") },
    // Users & Access
    { id: "users", label: "Users", icon: <UsersRound className="h-4 w-4" />, group: "users-access", keywords: "end user customer", onSelect: () => go("/users") },
    { id: "admins", label: "Admin Users", icon: <Users className="h-4 w-4" />, group: "users-access", keywords: "administrator staff", onSelect: () => go("/admins") },
    { id: "sessions", label: "Sessions", icon: <KeyRound className="h-4 w-4" />, group: "users-access", keywords: "token login active", onSelect: () => go("/sessions") },
    { id: "api-keys", label: "API Keys", icon: <KeySquare className="h-4 w-4" />, group: "users-access", keywords: "token bearer programmatic", onSelect: () => go("/api-keys") },
    { id: "roles", label: "Roles", icon: <Shield className="h-4 w-4" />, group: "users-access", keywords: "role scope permission access", onSelect: () => go("/roles") },
    // System
    { id: "cron", label: "Cron Jobs", icon: <Clock className="h-4 w-4" />, group: "system", keywords: "scheduler schedule task periodic", onSelect: () => go("/cron") },
    { id: "queue", label: "Job Queue", icon: <Inbox className="h-4 w-4" />, group: "system", keywords: "background worker task job", onSelect: () => go("/queue") },
    { id: "logs", label: "Logs", icon: <FileText className="h-4 w-4" />, group: "system", keywords: "log viewer stream tail", onSelect: () => go("/logs") },
    { id: "database", label: "Database", icon: <Database className="h-4 w-4" />, group: "system", keywords: "sqlite backup migration", onSelect: () => go("/database") },
    // Content
    { id: "uploads", label: "Uploads", icon: <Upload className="h-4 w-4" />, group: "content", keywords: "file media image", onSelect: () => go("/uploads") },
    { id: "notifications", label: "Notifications", icon: <Bell className="h-4 w-4" />, group: "content", keywords: "alert message notify", onSelect: () => go("/notifications") },
    { id: "audit", label: "Audit Log", icon: <ScrollText className="h-4 w-4" />, group: "content", keywords: "audit trail history activity", onSelect: () => go("/audit") },
    // Config
    { id: "settings", label: "Settings", icon: <Settings className="h-4 w-4" />, group: "config", keywords: "config configuration preference", onSelect: () => go("/settings") },
    // Quick actions
    {
      id: "theme-light",
      label: "Switch to Light Mode",
      icon: <Sun className="h-4 w-4" />,
      group: "actions",
      keywords: "theme appearance light",
      onSelect: () => { setTheme("light"); onClose(); },
    },
    {
      id: "theme-dark",
      label: "Switch to Dark Mode",
      icon: <Moon className="h-4 w-4" />,
      group: "actions",
      keywords: "theme appearance dark",
      onSelect: () => { setTheme("dark"); onClose(); },
    },
    {
      id: "theme-system",
      label: "Switch to System Theme",
      icon: <Monitor className="h-4 w-4" />,
      group: "actions",
      keywords: "theme appearance system auto",
      onSelect: () => { setTheme("system"); onClose(); },
    },
    {
      id: "logout",
      label: "Log Out",
      icon: <LogOut className="h-4 w-4" />,
      group: "actions",
      keywords: "sign out exit",
      onSelect: () => { onClose(); logout(); },
    },
  ];

  // Filter items by query (substring match on label + keywords).
  const filtered = query.trim()
    ? items.filter((item) => {
        const q = query.toLowerCase();
        const haystack = `${item.label} ${item.keywords || ""}`.toLowerCase();
        return q.split(/\s+/).every((word) => haystack.includes(word));
      })
    : items;

  // Filter out the current theme action (no point switching to what's already set).
  const visible = filtered.filter((item) => {
    if (item.id === "theme-light" && theme === "light") return false;
    if (item.id === "theme-dark" && theme === "dark") return false;
    if (item.id === "theme-system" && theme === "system") return false;
    return true;
  });

  const groups: { key: string; label: string; items: CommandItem[] }[] = [
    { key: "other", label: "General", items: visible.filter((i) => i.group === "other") },
    { key: "users-access", label: "Users & Access", items: visible.filter((i) => i.group === "users-access") },
    { key: "system", label: "System", items: visible.filter((i) => i.group === "system") },
    { key: "content", label: "Content", items: visible.filter((i) => i.group === "content") },
    { key: "config", label: "Config", items: visible.filter((i) => i.group === "config") },
    { key: "actions", label: "Quick Actions", items: visible.filter((i) => i.group === "actions") },
  ].filter((g) => g.items.length > 0);
  const flatList = groups.flatMap((g) => g.items);

  // Reset active index when filtered list changes.
  useEffect(() => {
    setActiveIndex(0);
  }, [query]);

  // Dialog open/close sync.
  useEffect(() => {
    const dialog = dialogRef.current;
    if (!dialog) return;
    if (open && !dialog.open) {
      dialog.showModal();
      setQuery("");
      setActiveIndex(0);
      // Small delay to ensure dialog is visible before focusing.
      requestAnimationFrame(() => inputRef.current?.focus());
    } else if (!open && dialog.open) {
      dialog.close();
    }
  }, [open]);

  // Scroll active item into view.
  useEffect(() => {
    if (!listRef.current) return;
    const active = listRef.current.querySelector("[data-active=\"true\"]");
    if (active) {
      active.scrollIntoView({ block: "nearest" });
    }
  }, [activeIndex]);

  function handleKeyDown(e: React.KeyboardEvent) {
    if (e.key === "ArrowDown") {
      e.preventDefault();
      setActiveIndex((i) => (i + 1) % flatList.length);
    } else if (e.key === "ArrowUp") {
      e.preventDefault();
      setActiveIndex((i) => (i - 1 + flatList.length) % flatList.length);
    } else if (e.key === "Enter") {
      e.preventDefault();
      if (flatList[activeIndex]) {
        flatList[activeIndex].onSelect();
      }
    }
  }

  function handleClose() {
    onClose();
  }

  function handleBackdropClick(e: React.MouseEvent<HTMLDialogElement>) {
    if (e.target === dialogRef.current) {
      onClose();
    }
  }

  let itemIndex = -1;

  return (
    <dialog
      ref={dialogRef}
      onClose={handleClose}
      onClick={handleBackdropClick}
      className="backdrop:bg-black/50 bg-transparent p-0 m-auto mt-[20vh] max-h-[60vh] overflow-visible"
    >
      <div
        className="bg-card text-card-foreground rounded-lg shadow-lg w-full max-w-lg border border-border"
        onKeyDown={handleKeyDown}
      >
        {/* Search input */}
        <div className="flex items-center gap-2 px-3 border-b border-border">
          <Search className="h-4 w-4 shrink-0 text-muted-foreground" />
          <input
            ref={inputRef}
            type="text"
            placeholder="Type a command or search..."
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            className="flex-1 bg-transparent py-3 text-sm outline-none placeholder:text-muted-foreground"
          />
          <kbd className="hidden sm:inline-flex h-5 items-center rounded border border-border bg-muted px-1.5 text-[10px] font-medium text-muted-foreground">
            ESC
          </kbd>
        </div>

        {/* Results */}
        <div ref={listRef} className="max-h-[50vh] overflow-y-auto p-1">
          {flatList.length === 0 && (
            <p className="py-6 text-center text-sm text-muted-foreground">
              No results found.
            </p>
          )}

          {groups.map((group, groupIdx) => (
            <div key={group.key}>
              <div className={cn("px-2 py-1.5 text-xs font-medium text-muted-foreground", groupIdx > 0 && "mt-1 border-t border-border pt-2")}>
                {group.label}
              </div>
              {group.items.map((item) => {
                itemIndex++;
                const idx = itemIndex;
                return (
                  <button
                    key={item.id}
                    data-active={idx === activeIndex}
                    className={cn(
                      "flex w-full items-center gap-3 rounded-md px-2 py-2 text-sm transition-colors",
                      idx === activeIndex
                        ? "bg-accent text-accent-foreground"
                        : "text-foreground hover:bg-accent/50"
                    )}
                    onClick={() => item.onSelect()}
                    onMouseEnter={() => setActiveIndex(idx)}
                  >
                    <span className="shrink-0 text-muted-foreground">{item.icon}</span>
                    <span>{item.label}</span>
                  </button>
                );
              })}
            </div>
          ))}
        </div>
      </div>
    </dialog>
  );
}
