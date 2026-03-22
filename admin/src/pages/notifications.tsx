import { useCallback, useEffect, useState } from "react";
import { toast } from "sonner";
import { get, post, del, downloadCSV } from "@/lib/api";
import { useSelection } from "@/lib/use-selection";
import { Button } from "@/components/ui/button";
import {
  Check,
  CheckCheck,
  Download,
  Trash2,
  Bell,
  Filter,
  X,
} from "lucide-react";
import { TableSkeleton } from "@/components/ui/skeleton";
import { ErrorAlert } from "@/components/ui/error-alert";
import { Pagination } from "@/components/ui/pagination";
import { TableEmptyRow } from "@/components/ui/empty-state";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { BulkActionBar } from "@/components/ui/bulk-action-bar";
import { SortableHeader, useSort } from "@/components/ui/sortable-header";
import { ColumnToggle } from "@/components/ui/column-toggle";
import { useColumnVisibility } from "@/lib/use-column-visibility";
import { cn } from "@/lib/utils";

const NOTIFICATION_COLUMNS = [
  { key: "notification", label: "Notification" },
  { key: "type", label: "Type" },
  { key: "time", label: "Time" },
];

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
  info: "bg-blue-100 text-blue-700 dark:bg-blue-500/10 dark:text-blue-400",
  success: "bg-green-100 text-green-700 dark:bg-green-500/10 dark:text-green-400",
  warning: "bg-amber-100 text-amber-700 dark:bg-amber-500/10 dark:text-amber-400",
  error: "bg-red-100 text-red-700 dark:bg-red-500/10 dark:text-red-400",
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
  const [deleteTarget, setDeleteTarget] = useState<Notification | null>(null);
  const [exporting, setExporting] = useState(false);

  // Pagination.
  const [page, setPage] = useState(0);
  const pageSize = 20;

  // Filter.
  const [unreadOnly, setUnreadOnly] = useState(false);

  // Sort.
  const [sort, toggleSort] = useSort("id", "desc");

  // Column visibility.
  const { isVisible, toggle: toggleColumn, visibleCount, columns: colDefs } = useColumnVisibility("notifications", NOTIFICATION_COLUMNS);

  // Selection.
  const selection = useSelection<number>();
  const [bulkDeleting, setBulkDeleting] = useState(false);
  const [bulkConfirmOpen, setBulkConfirmOpen] = useState(false);

  const load = useCallback(async () => {
    try {
      const params = new URLSearchParams();
      params.set("limit", String(pageSize));
      params.set("offset", String(page * pageSize));
      if (unreadOnly) params.set("unread", "true");
      params.set("sort", sort.column);
      params.set("order", sort.direction);

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
  }, [page, unreadOnly, sort.column, sort.direction]);

  useEffect(() => {
    load();
  }, [load]);

  // Clear selection when page, filter, or sort changes.
  useEffect(() => {
    selection.clear();
  }, [page, unreadOnly, sort.column, sort.direction]);

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

  async function deleteNotification() {
    if (!deleteTarget) return;
    const id = deleteTarget.id;
    setActing(true);
    try {
      await del(`/admin/notifications/${id}`);
      setNotifications((prev) => prev.filter((n) => n.id !== id));
      setTotal((t) => Math.max(0, t - 1));
      if (!deleteTarget.read_at) {
        setUnread((c) => Math.max(0, c - 1));
      }
      setDeleteTarget(null);
      toast.success("Notification deleted");
    } catch (e: any) {
      toast.error(e.message || "Failed to delete notification");
    } finally {
      setActing(false);
    }
  }

  async function handleBulkDelete() {
    setBulkDeleting(true);
    try {
      const data = await post<{ affected: number }>("/admin/notifications/bulk-delete", { ids: selection.ids });
      setBulkConfirmOpen(false);
      selection.clear();
      toast.success(`${data.affected} notification${data.affected !== 1 ? "s" : ""} deleted`);
      await load();
    } catch (e: any) {
      toast.error(e.message || "Failed to bulk delete notifications");
    } finally {
      setBulkDeleting(false);
    }
  }

  async function handleExport() {
    setExporting(true);
    try {
      const params = new URLSearchParams();
      if (unreadOnly) params.set("unread", "true");
      params.set("sort", sort.column);
      params.set("order", sort.direction);
      await downloadCSV(`/admin/notifications/export?${params}`);
    } catch {
      toast.error("Failed to export notifications");
    } finally {
      setExporting(false);
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
        <div className="flex items-center gap-2">
          <ColumnToggle columns={colDefs} isVisible={isVisible} toggle={toggleColumn} />
          <Button variant="outline" size="sm" onClick={handleExport} disabled={exporting}>
            <Download className="h-4 w-4 mr-2" />
            {exporting ? "Exporting..." : "Export CSV"}
          </Button>
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
              <th className="p-3 w-10">
                <input
                  type="checkbox"
                  checked={selection.isAllSelected(notifications.map((n) => n.id))}
                  onChange={() => selection.toggleAll(notifications.map((n) => n.id))}
                  className="rounded border-input"
                />
              </th>
              <th className="text-left p-3 font-medium w-8"></th>
              {isVisible("notification") && <th className="text-left p-3 font-medium">Notification</th>}
              {isVisible("type") && <SortableHeader label="Type" column="type" sort={sort} onSort={toggleSort} className="hidden md:table-cell" />}
              {isVisible("time") && <SortableHeader label="Time" column="created_at" sort={sort} onSort={toggleSort} className="hidden sm:table-cell" />}
              <th className="text-right p-3 font-medium w-24">Actions</th>
            </tr>
          </thead>
          <tbody>
            {notifications.length === 0 ? (
              <TableEmptyRow
                colSpan={visibleCount + 3}
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
                    !n.read_at && "bg-muted/20",
                    selection.isSelected(n.id) && "bg-muted/40"
                  )}
                >
                  <td className="p-3">
                    <input
                      type="checkbox"
                      checked={selection.isSelected(n.id)}
                      onChange={() => selection.toggle(n.id)}
                      className="rounded border-input"
                    />
                  </td>
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
                  {isVisible("notification") && (
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
                  )}

                  {/* Type badge */}
                  {isVisible("type") && (
                    <td className="p-3 hidden md:table-cell">
                      <span
                        className={cn(
                          "inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium",
                          TYPE_COLORS[n.type] || "bg-gray-100 text-gray-700 dark:bg-gray-500/10 dark:text-gray-400"
                        )}
                      >
                        {n.type}
                      </span>
                    </td>
                  )}

                  {/* Time */}
                  {isVisible("time") && (
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
                  )}

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
                        onClick={() => setDeleteTarget(n)}
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

      {/* Delete Confirmation */}
      <ConfirmDialog
        open={!!deleteTarget}
        onClose={() => setDeleteTarget(null)}
        onConfirm={deleteNotification}
        title="Delete Notification"
        message="Are you sure you want to delete this notification?"
        confirmLabel="Delete"
        loading={acting}
        details={deleteTarget && (
          <>
            <div><span className="font-medium">Title:</span> {deleteTarget.title}</div>
            <div className="text-xs text-muted-foreground line-clamp-2">{deleteTarget.message}</div>
          </>
        )}
      />

      {/* Bulk Actions */}
      <BulkActionBar count={selection.count} onClear={selection.clear}>
        <Button variant="destructive" size="sm" onClick={() => setBulkConfirmOpen(true)}>
          <Trash2 className="h-3.5 w-3.5 mr-1" />
          Delete
        </Button>
      </BulkActionBar>

      <ConfirmDialog
        open={bulkConfirmOpen}
        onClose={() => setBulkConfirmOpen(false)}
        onConfirm={handleBulkDelete}
        title="Delete Notifications"
        message={`Are you sure you want to delete ${selection.count} notification${selection.count !== 1 ? "s" : ""}? This action cannot be undone.`}
        confirmLabel="Delete"
        loading={bulkDeleting}
      />
    </div>
  );
}
