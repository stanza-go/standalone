import { useState, useEffect, useCallback, useRef } from "react";
import { useNavigate } from "react-router";
import { get, post, del } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Bell, Check, CheckCheck, Trash2, ExternalLink } from "lucide-react";
import { cn } from "@/lib/utils";

interface Notification {
  id: number;
  type: string;
  title: string;
  message: string;
  read_at?: string;
  created_at: string;
}

interface ListResponse {
  notifications: Notification[];
  total: number;
  unread: number;
}

function formatRelativeTime(iso: string): string {
  const diff = (Date.now() - new Date(iso).getTime()) / 1000;
  if (diff < 60) return "just now";
  if (diff < 3600) return `${Math.round(diff / 60)}m ago`;
  if (diff < 86400) return `${Math.round(diff / 3600)}h ago`;
  return `${Math.round(diff / 86400)}d ago`;
}

const TYPE_COLORS: Record<string, string> = {
  info: "bg-blue-500",
  success: "bg-green-500",
  warning: "bg-amber-500",
  error: "bg-red-500",
};

export function NotificationBell({ collapsed }: { collapsed?: boolean }) {
  const navigate = useNavigate();
  const [unreadCount, setUnreadCount] = useState(0);
  const [notifications, setNotifications] = useState<Notification[]>([]);
  const [open, setOpen] = useState(false);
  const [loading, setLoading] = useState(false);
  const ref = useRef<HTMLDivElement>(null);

  // Fetch unread count.
  const fetchCount = useCallback(async () => {
    try {
      const data = await get<{ unread: number }>("/admin/notifications/unread-count");
      setUnreadCount(data.unread);
    } catch {
      // Silently fail — non-critical.
    }
  }, []);

  // Fetch recent notifications for dropdown.
  const fetchRecent = useCallback(async () => {
    setLoading(true);
    try {
      const data = await get<ListResponse>("/admin/notifications?limit=10&offset=0");
      setNotifications(data.notifications);
      setUnreadCount(data.unread);
    } catch {
      // Silently fail.
    } finally {
      setLoading(false);
    }
  }, []);

  // Poll unread count every 30s.
  useEffect(() => {
    fetchCount();
    const interval = setInterval(() => {
      if (document.visibilityState === "visible") {
        fetchCount();
      }
    }, 30_000);
    return () => clearInterval(interval);
  }, [fetchCount]);

  // Fetch notifications when dropdown opens.
  useEffect(() => {
    if (open) fetchRecent();
  }, [open, fetchRecent]);

  // Close on outside click.
  useEffect(() => {
    if (!open) return;
    function handleClick(e: MouseEvent) {
      if (ref.current && !ref.current.contains(e.target as Node)) {
        setOpen(false);
      }
    }
    document.addEventListener("mousedown", handleClick);
    return () => document.removeEventListener("mousedown", handleClick);
  }, [open]);

  async function markRead(id: number) {
    try {
      await post(`/admin/notifications/${id}/read`);
      setNotifications((prev) =>
        prev.map((n) => (n.id === id ? { ...n, read_at: new Date().toISOString() } : n))
      );
      setUnreadCount((c) => Math.max(0, c - 1));
    } catch {
      // Silently fail.
    }
  }

  async function markAllRead() {
    try {
      await post("/admin/notifications/read-all");
      setNotifications((prev) => prev.map((n) => ({ ...n, read_at: n.read_at || new Date().toISOString() })));
      setUnreadCount(0);
    } catch {
      // Silently fail.
    }
  }

  async function deleteNotification(id: number) {
    try {
      await del(`/admin/notifications/${id}`);
      setNotifications((prev) => prev.filter((n) => n.id !== id));
      const deleted = notifications.find((n) => n.id === id);
      if (deleted && !deleted.read_at) {
        setUnreadCount((c) => Math.max(0, c - 1));
      }
    } catch {
      // Silently fail.
    }
  }

  return (
    <div ref={ref} className="relative">
      <Button
        variant="ghost"
        size="icon"
        className="relative h-8 w-8"
        onClick={() => setOpen(!open)}
        title="Notifications"
      >
        <Bell className="h-4 w-4" />
        {unreadCount > 0 && (
          <span className="absolute -top-0.5 -right-0.5 flex h-4 min-w-4 items-center justify-center rounded-full bg-destructive px-1 text-[10px] font-bold text-destructive-foreground">
            {unreadCount > 99 ? "99+" : unreadCount}
          </span>
        )}
      </Button>

      {open && (
        <div
          className={cn(
            "absolute z-50 mt-1 w-80 rounded-lg border border-border bg-background shadow-lg",
            collapsed === false ? "right-0" : "left-0 md:right-0 md:left-auto"
          )}
        >
          {/* Header */}
          <div className="flex items-center justify-between border-b border-border px-4 py-3">
            <h3 className="text-sm font-semibold">Notifications</h3>
            <div className="flex items-center gap-1">
              {unreadCount > 0 && (
                <Button
                  variant="ghost"
                  size="sm"
                  className="h-7 text-xs"
                  onClick={markAllRead}
                  title="Mark all as read"
                >
                  <CheckCheck className="h-3.5 w-3.5 mr-1" />
                  Read all
                </Button>
              )}
            </div>
          </div>

          {/* List */}
          <div className="max-h-80 overflow-y-auto">
            {loading && notifications.length === 0 ? (
              <div className="p-6 text-center text-sm text-muted-foreground">
                Loading...
              </div>
            ) : notifications.length === 0 ? (
              <div className="p-6 text-center text-sm text-muted-foreground">
                No notifications yet
              </div>
            ) : (
              notifications.map((n) => (
                <div
                  key={n.id}
                  className={cn(
                    "group flex gap-3 border-b border-border px-4 py-3 last:border-0",
                    !n.read_at && "bg-muted/40"
                  )}
                >
                  {/* Dot indicator */}
                  <div className="mt-1.5 shrink-0">
                    <span
                      className={cn(
                        "block h-2 w-2 rounded-full",
                        n.read_at ? "bg-transparent" : (TYPE_COLORS[n.type] || "bg-blue-500")
                      )}
                    />
                  </div>

                  {/* Content */}
                  <div className="min-w-0 flex-1">
                    <p className="text-sm font-medium leading-snug">{n.title}</p>
                    <p className="mt-0.5 text-xs text-muted-foreground line-clamp-2">
                      {n.message}
                    </p>
                    <p className="mt-1 text-[11px] text-muted-foreground/70">
                      {formatRelativeTime(n.created_at)}
                    </p>
                  </div>

                  {/* Actions */}
                  <div className="flex shrink-0 items-start gap-0.5 opacity-0 group-hover:opacity-100 transition-opacity">
                    {!n.read_at && (
                      <button
                        onClick={() => markRead(n.id)}
                        className="rounded p-1 hover:bg-muted"
                        title="Mark as read"
                      >
                        <Check className="h-3.5 w-3.5 text-muted-foreground" />
                      </button>
                    )}
                    <button
                      onClick={() => deleteNotification(n.id)}
                      className="rounded p-1 hover:bg-muted"
                      title="Delete"
                    >
                      <Trash2 className="h-3.5 w-3.5 text-muted-foreground" />
                    </button>
                  </div>
                </div>
              ))
            )}
          </div>

          {/* Footer */}
          <div className="border-t border-border px-4 py-2">
            <button
              onClick={() => {
                setOpen(false);
                navigate("/notifications");
              }}
              className="flex w-full items-center justify-center gap-1.5 rounded py-1 text-xs font-medium text-muted-foreground hover:text-foreground transition-colors"
            >
              View all notifications
              <ExternalLink className="h-3 w-3" />
            </button>
          </div>
        </div>
      )}
    </div>
  );
}
