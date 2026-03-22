import { useCallback, useEffect, useState } from "react";
import { toast } from "sonner";
import { get, del, post, downloadCSV } from "@/lib/api";
import { useSelection } from "@/lib/use-selection";
import { Button } from "@/components/ui/button";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { BulkActionBar } from "@/components/ui/bulk-action-bar";
import { TableSkeleton } from "@/components/ui/skeleton";
import { ErrorAlert } from "@/components/ui/error-alert";
import { TableEmptyRow } from "@/components/ui/empty-state";
import { SortableHeader, useSort } from "@/components/ui/sortable-header";
import { ColumnToggle } from "@/components/ui/column-toggle";
import { useColumnVisibility } from "@/lib/use-column-visibility";
import { Download, Trash2 } from "lucide-react";

const SESSION_COLUMNS = [
  { key: "token_id", label: "Token ID" },
  { key: "type", label: "Type" },
  { key: "admin", label: "Admin" },
  { key: "created_at", label: "Created" },
  { key: "expires_at", label: "Expires" },
];

interface Session {
  id: string;
  entity_type: string;
  entity_id: string;
  email: string;
  name: string;
  created_at: string;
  expires_at: string;
}

export default function SessionsPage() {
  const [sessions, setSessions] = useState<Session[]>([]);
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(true);
  const [acting, setActing] = useState<string | null>(null);
  const [exporting, setExporting] = useState(false);

  // Sort.
  const [sort, toggleSort] = useSort("created_at", "desc");

  // Column visibility.
  const { isVisible, toggle: toggleColumn, visibleCount, columns: colDefs } = useColumnVisibility("sessions", SESSION_COLUMNS);

  // Selection.
  const selection = useSelection<string>();
  const [bulkRevoking, setBulkRevoking] = useState(false);
  const [bulkConfirmOpen, setBulkConfirmOpen] = useState(false);

  // Revoke confirmation.
  const [revokeTarget, setRevokeTarget] = useState<Session | null>(null);

  const load = useCallback(async () => {
    try {
      const data = await get<{ sessions: Session[] }>(`/admin/sessions?sort=${sort.column}&order=${sort.direction}`);
      setSessions(data.sessions);
      setError("");
    } catch (e: any) {
      setError(e.message || "Failed to load sessions");
    } finally {
      setLoading(false);
    }
  }, [sort.column, sort.direction]);

  useEffect(() => {
    load();
    const interval = setInterval(load, 10000);
    return () => clearInterval(interval);
  }, [load]);

  // Clear selection when sort changes.
  useEffect(() => {
    selection.clear();
  }, [sort.column, sort.direction]);

  async function handleExport() {
    setExporting(true);
    try {
      const params = new URLSearchParams();
      params.set("sort", sort.column);
      params.set("order", sort.direction);
      await downloadCSV(`/admin/sessions/export?${params}`);
    } catch {
      toast.error("Failed to export sessions");
    } finally {
      setExporting(false);
    }
  }

  async function handleRevoke() {
    if (!revokeTarget) return;
    const id = revokeTarget.id;
    setActing(id);
    try {
      await del(`/admin/sessions/${id}`);
      setRevokeTarget(null);
      toast.success("Session revoked");
      await load();
    } catch (e: any) {
      toast.error(e.message || "Failed to revoke session");
    } finally {
      setActing(null);
    }
  }

  async function handleBulkRevoke() {
    setBulkRevoking(true);
    try {
      const data = await post<{ affected: number }>("/admin/sessions/bulk-revoke", { ids: selection.ids });
      setBulkConfirmOpen(false);
      selection.clear();
      toast.success(`${data.affected} session${data.affected !== 1 ? "s" : ""} revoked`);
      await load();
    } catch (e: any) {
      toast.error(e.message || "Failed to bulk revoke sessions");
    } finally {
      setBulkRevoking(false);
    }
  }

  function formatTime(iso: string): string {
    if (!iso) return "\u2014";
    const d = new Date(iso);
    return d.toLocaleDateString() + " " + d.toLocaleTimeString();
  }

  function relativeTime(iso: string): string {
    if (!iso) return "";
    const d = new Date(iso);
    const now = new Date();
    const diff = d.getTime() - now.getTime();
    const absDiff = Math.abs(diff);
    const minutes = Math.floor(absDiff / 60000);
    const hours = Math.floor(minutes / 60);

    if (diff > 0) {
      if (hours > 0) return `in ${hours}h ${minutes % 60}m`;
      return `in ${minutes}m`;
    }
    if (hours > 0) return `${hours}h ${minutes % 60}m ago`;
    return `${minutes}m ago`;
  }

  if (loading) {
    return (
      <div className="p-6">
        <h1 className="text-2xl font-bold mb-6">Active Sessions</h1>
        <TableSkeleton columns={[
          { width: "w-20", hidden: "hidden md:table-cell" },
          { width: "w-16" },
          { width: "w-24" },
          { width: "w-24", hidden: "hidden md:table-cell" },
          { width: "w-20" },
          { width: "w-16" },
        ]} />
      </div>
    );
  }

  return (
    <div className="p-6">
      <div className="mb-6 flex items-start justify-between">
        <div>
          <h1 className="text-2xl font-bold">Active Sessions</h1>
          <p className="text-sm text-muted-foreground mt-1">
            {sessions.length} active session{sessions.length !== 1 ? "s" : ""}
          </p>
        </div>
        <div className="flex gap-2">
          <ColumnToggle columns={colDefs} isVisible={isVisible} toggle={toggleColumn} />
          <Button variant="outline" onClick={handleExport} disabled={exporting}>
            <Download className="h-4 w-4 mr-2" />
            {exporting ? "Exporting..." : "Export CSV"}
          </Button>
        </div>
      </div>

      {error && (
        <ErrorAlert message={error} onRetry={load} onDismiss={() => setError("")} className="mb-4" />
      )}

      <div className="border rounded-lg overflow-hidden">
        <table className="w-full text-sm">
          <thead>
            <tr className="bg-muted/50 border-b">
              <th className="p-3 w-10">
                <input
                  type="checkbox"
                  checked={selection.isAllSelected(sessions.map((s) => s.id))}
                  onChange={() => selection.toggleAll(sessions.map((s) => s.id))}
                  className="rounded border-input"
                />
              </th>
              {isVisible("token_id") && <th className="text-left p-3 font-medium hidden md:table-cell">Token ID</th>}
              {isVisible("type") && <SortableHeader label="Type" column="entity_type" sort={sort} onSort={toggleSort} />}
              {isVisible("admin") && <th className="text-left p-3 font-medium">Admin</th>}
              {isVisible("created_at") && <SortableHeader label="Created" column="created_at" sort={sort} onSort={toggleSort} className="hidden md:table-cell" />}
              {isVisible("expires_at") && <SortableHeader label="Expires" column="expires_at" sort={sort} onSort={toggleSort} />}
              <th className="text-right p-3 font-medium">Actions</th>
            </tr>
          </thead>
          <tbody>
            {sessions.length === 0 ? (
              <TableEmptyRow colSpan={visibleCount + 2} message="No active sessions" />
            ) : (
              sessions.map((session) => (
                <tr
                  key={session.id}
                  className={`border-b last:border-0 hover:bg-muted/30 ${selection.isSelected(session.id) ? "bg-muted/40" : ""}`}
                >
                  <td className="p-3">
                    <input
                      type="checkbox"
                      checked={selection.isSelected(session.id)}
                      onChange={() => selection.toggle(session.id)}
                      className="rounded border-input"
                    />
                  </td>
                  {isVisible("token_id") && (
                    <td className="p-3 font-mono text-xs hidden md:table-cell">
                      {session.id.substring(0, 12)}...
                    </td>
                  )}
                  {isVisible("type") && (
                    <td className="p-3">
                      <span className="inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium bg-blue-100 text-blue-700 dark:bg-blue-500/10 dark:text-blue-400">
                        {session.entity_type}
                      </span>
                    </td>
                  )}
                  {isVisible("admin") && (
                    <td className="p-3">
                      <div>{session.name || "\u2014"}</div>
                      <div className="text-xs text-muted-foreground">
                        {session.email}
                      </div>
                    </td>
                  )}
                  {isVisible("created_at") && (
                    <td className="p-3 text-muted-foreground text-xs hidden md:table-cell">
                      <div>{formatTime(session.created_at)}</div>
                      <div>{relativeTime(session.created_at)}</div>
                    </td>
                  )}
                  {isVisible("expires_at") && (
                    <td className="p-3 text-muted-foreground text-xs">
                      <div>{formatTime(session.expires_at)}</div>
                      <div>{relativeTime(session.expires_at)}</div>
                    </td>
                  )}
                  <td className="p-3 text-right">
                    <Button
                      variant="destructive"
                      size="sm"
                      onClick={() => setRevokeTarget(session)}
                    >
                      Revoke
                    </Button>
                  </td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>

      {/* Revoke Confirmation */}
      <ConfirmDialog
        open={!!revokeTarget}
        onClose={() => setRevokeTarget(null)}
        onConfirm={handleRevoke}
        title="Revoke Session"
        message="Are you sure you want to revoke this session? The user will be logged out immediately."
        confirmLabel="Revoke Session"
        loading={acting === revokeTarget?.id}
        details={revokeTarget && (
          <>
            <div><span className="font-medium">User:</span> {revokeTarget.name || revokeTarget.email}</div>
            <div><span className="font-medium">Type:</span> {revokeTarget.entity_type}</div>
            <div><span className="font-medium">Token:</span> <span className="font-mono text-xs">{revokeTarget.id.substring(0, 16)}...</span></div>
          </>
        )}
      />

      {/* Bulk Actions */}
      <BulkActionBar count={selection.count} onClear={selection.clear}>
        <Button variant="destructive" size="sm" onClick={() => setBulkConfirmOpen(true)}>
          <Trash2 className="h-3.5 w-3.5 mr-1" />
          Revoke
        </Button>
      </BulkActionBar>

      <ConfirmDialog
        open={bulkConfirmOpen}
        onClose={() => setBulkConfirmOpen(false)}
        onConfirm={handleBulkRevoke}
        title="Revoke Sessions"
        message={`Are you sure you want to revoke ${selection.count} session${selection.count !== 1 ? "s" : ""}? This action cannot be undone.`}
        confirmLabel="Revoke"
        loading={bulkRevoking}
      />
    </div>
  );
}
