import { useCallback, useEffect, useState } from "react";
import { toast } from "sonner";
import { get, post, del } from "@/lib/api";
import { Button } from "@/components/ui/button";
import {
  Check,
  CheckCheck,
  Trash2,
  Bell,
  Filter,
  X,
} from "lucide-react";
import { TableSkeleton } from "@/components/ui/skeleton";
import { ErrorAlert } from "@/components/ui/error-alert";
import { Pagination } from "@/components/ui/pagination";
import { TableEmptyRow } from "@/components/ui/empty-state";
import { cn } from "@/lib/utils";

interface Notification {
  id: number;
  entity_type: string;
  entity_id: number;
  type: string;
  title: string;
  message: string;
  data?: string;
  read_at?: string;
  created_at: string;
}

interface ListResponse {
  notifications: Notification[];
  total: number;
  unread: number;
}

function formatTime(iso: string): string {
  if (!iso) return "\u2014";
  const d = new Date(iso);
  return d.toLocaleDateString() + " " + d.toLocaleTimeString();
}

function formatRelativeTime(iso: string): string {
  const diff = (Date.now() - new Date(iso).getTime()) / 1000;
  if (diff < 60) return "just now";
  if (diff < 3600) return `${Math.round(diff / 60)}m ago`;
  if (diff < 86400) return `${Math.round(diff / 3600)}h ago`;
  return `${Math.round(diff / 86400)}d ago`;
}

const TYPE_COLORS: Record<string, string> = {
  info: "bg-blue-100 text-blue-700",
  success: "bg-green-100 text-green-700",
  warning: "bg-amber-100 text-amber-700",
  error: "bg-red-100 text-red-700",
};

const TYPE_DOT: Record<string, string> = {
  info: "bg-blue-500",
  success: "bg-green-500",
  warning: "bg-amber-500",
  error: "bg-red-500",
};

export default function NotificationsPage() {
  const [notifications, setNotifications] = useState<Notification[]>([]);
  const [total, setTotal] = useState(0);
  const [unread, setUnread] = useState(0);
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(true);
  const [acting, setActing] = useState(false);

  // Pagination.
  const [page, setPage] = useState(0);
  const pageSize = 20;

  // Filter.
  const [unreadOnly, setUnreadOnly] = useState(false);

  const load = useCallback(async () => {
    try {
      const params = new URLSearchParams();
      params.set("limit", String(pageSize));
      params.set("offset", String(page * pageSize));
      if (unreadOnly) params.set("unread", "true");

      const data = await get<ListResponse>(`/admin/notifications?${params}`);
      setNotifications(data.notifications);
      setTotal(data.total);
      setUnread(data.unread);
      setError("");
    } catch (e: any) {
      setError(e.message || "Failed to load notifications");
    } finally {
      setLoading(false);
    }
  }, [page, unreadOnly]);

  useEffect(() => {
    load();
  }, [load]);

  async function markRead(id: number) {
    setActing(true);
    try {
      await post(`/admin/notifications/${id}/read`);
      setNotifications((prev) =>
        prev.map((n) => (n.id === id ? { ...n, read_at: new Date().toISOString() } : n))
      );
      setUnread((c) => Math.max(0, c - 1));
    } catch (e: any) {
      toast.error(e.message || "Failed to mark as read");
    } finally {
      setActing(false);
    }
  }

  async function markAllRead() {
    setActing(true);
    try {
      await post("/admin/notifications/read-all");
      setNotifications((prev) =>
        prev.map((n) => ({ ...n, read_at: n.read_at || new Date().toISOString() }))
      );
      setUnread(0);
      toast.success("All notifications marked as read");
    } catch (e: any) {
      toast.error(e.message || "Failed to mark all as read");
    } finally {
      setActing(false);
    }
  }

  async function deleteNotification(id: number) {
    setActing(true);
    try {
      await del(`/admin/notifications/${id}`);
      const deleted = notifications.find((n) => n.id === id);
      setNotifications((prev) => prev.filter((n) => n.id !== id));
      setTotal((t) => Math.max(0, t - 1));
      if (deleted && !deleted.read_at) {
        setUnread((c) => Math.max(0, c - 1));
      }
      toast.success("Notification deleted");
    } catch (e: any) {
      toast.error(e.message || "Failed to delete notification");
    } finally {
      setActing(false);
    }
  }

  const totalPages = Math.ceil(total / pageSize);

  if (loading) {
    return (
      <div className="p-6">
        <h1 className="text-2xl font-bold mb-6">Notifications</h1>
        <TableSkeleton columns={[
          { width: "w-6" },
          { width: "w-48" },
          { width: "w-16", hidden: "hidden md:table-cell" },
          { width: "w-20", hidden: "hidden sm:table-cell" },
          { width: "w-20" },
        ]} />
      </div>
    );
  }

  return (
    <div className="p-6">
      <div className="mb-6 flex flex-wrap items-start justify-between gap-4">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight">Notifications</h1>
          <p className="text-sm text-muted-foreground">
            {total} notification{total !== 1 ? "s" : ""}
            {unread > 0 && ` \u00B7 ${unread} unread`}
          </p>
        </div>
        {unread > 0 && (
          <Button
            variant="outline"
            size="sm"
            disabled={acting}
            onClick={markAllRead}
          >
            <CheckCheck className="h-4 w-4 mr-1.5" />
            Mark all as read
          </Button>
        )}
      </div>

      {error && (
        <ErrorAlert
          message={error}
          onRetry={load}
          onDismiss={() => setError("")}
          className="mb-4"
        />
      )}

      {/* Filter */}
      <div className="mb-4 flex items-center gap-2">
        <Filter className="h-4 w-4 text-muted-foreground" />
        <select
          value={unreadOnly ? "unread" : "all"}
          onChange={(e) => {
            setUnreadOnly(e.target.value === "unread");
            setPage(0);
          }}
          className="h-9 rounded-md border border-input bg-background px-3 text-sm"
        >
          <option value="all">All notifications</option>
          <option value="unread">Unread only</option>
        </select>

        {unreadOnly && (
          <Button
            variant="ghost"
            size="sm"
            onClick={() => {
              setUnreadOnly(false);
              setPage(0);
            }}
          >
            <X className="h-4 w-4 mr-1" />
            Clear filter
          </Button>
        )}
      </div>

      {/* Notification list */}
      <div className="border rounded-lg overflow-hidden">
        <table className="w-full text-sm">
          <thead>
            <tr className="bg-muted/50 border-b">
              <th className="text-left p-3 font-medium w-8"></th>
              <th className="text-left p-3 font-medium">Notification</th>
              <th className="text-left p-3 font-medium hidden md:table-cell">Type</th>
              <th className="text-left p-3 font-medium hidden sm:table-cell">Time</th>
              <th className="text-right p-3 font-medium w-24">Actions</th>
            </tr>
          </thead>
          <tbody>
            {notifications.length === 0 ? (
              <TableEmptyRow
                colSpan={5}
                message={
                  unreadOnly
                    ? "No unread notifications"
                    : "No notifications yet"
                }
              />
            ) : (
              notifications.map((n) => (
                <tr
                  key={n.id}
                  className={cn(
                    "border-b last:border-0 hover:bg-muted/30",
                    !n.read_at && "bg-muted/20"
                  )}
                >
                  {/* Unread dot */}
                  <td className="p-3">
                    {!n.read_at ? (
                      <span
                        className={cn(
                          "block h-2.5 w-2.5 rounded-full",
                          TYPE_DOT[n.type] || "bg-blue-500"
                        )}
                      />
                    ) : (
                      <Bell className="h-3.5 w-3.5 text-muted-foreground/40" />
                    )}
                  </td>

                  {/* Content */}
                  <td className="p-3">
                    <p className={cn("text-sm", !n.read_at && "font-medium")}>
                      {n.title}
                    </p>
                    <p className="mt-0.5 text-xs text-muted-foreground line-clamp-2">
                      {n.message}
                    </p>
                    {/* Show time on mobile (hidden on sm+) */}
                    <p className="mt-1 text-[11px] text-muted-foreground/70 sm:hidden">
                      {formatRelativeTime(n.created_at)}
                    </p>
                  </td>

                  {/* Type badge */}
                  <td className="p-3 hidden md:table-cell">
                    <span
                      className={cn(
                        "inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium",
                        TYPE_COLORS[n.type] || "bg-gray-100 text-gray-700"
                      )}
                    >
                      {n.type}
                    </span>
                  </td>

                  {/* Time */}
                  <td className="p-3 hidden sm:table-cell">
                    <div
                      className="text-xs text-muted-foreground"
                      title={formatTime(n.created_at)}
                    >
                      {formatRelativeTime(n.created_at)}
                    </div>
                    {n.read_at && (
                      <div className="text-[11px] text-muted-foreground/60 mt-0.5">
                        Read {formatRelativeTime(n.read_at)}
                      </div>
                    )}
                  </td>

                  {/* Actions */}
                  <td className="p-3 text-right">
                    <div className="flex items-center justify-end gap-1">
                      {!n.read_at && (
                        <Button
                          variant="ghost"
                          size="icon"
                          className="h-7 w-7"
                          disabled={acting}
                          onClick={() => markRead(n.id)}
                          title="Mark as read"
                        >
                          <Check className="h-3.5 w-3.5" />
                        </Button>
                      )}
                      <Button
                        variant="ghost"
                        size="icon"
                        className="h-7 w-7 text-muted-foreground hover:text-destructive"
                        disabled={acting}
                        onClick={() => deleteNotification(n.id)}
                        title="Delete"
                      >
                        <Trash2 className="h-3.5 w-3.5" />
                      </Button>
                    </div>
                  </td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>

      {/* Pagination */}
      <Pagination
        page={page}
        totalPages={totalPages}
        total={total}
        pageSize={pageSize}
        onPrev={() => setPage(page - 1)}
        onNext={() => setPage(page + 1)}
      />
    </div>
  );
}
