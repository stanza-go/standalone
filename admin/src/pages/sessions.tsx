import { useCallback, useEffect, useState } from "react";
import { toast } from "sonner";
import { get, del } from "@/lib/api";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogHeader,
  DialogTitle,
  DialogCloseButton,
  DialogBody,
  DialogFooter,
} from "@/components/ui/dialog";
import { TableSkeleton } from "@/components/ui/skeleton";
import { ErrorAlert } from "@/components/ui/error-alert";
import { TableEmptyRow } from "@/components/ui/empty-state";

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

  // Revoke confirmation.
  const [revokeTarget, setRevokeTarget] = useState<Session | null>(null);

  const load = useCallback(async () => {
    try {
      const data = await get<{ sessions: Session[] }>("/admin/sessions");
      setSessions(data.sessions);
      setError("");
    } catch (e: any) {
      setError(e.message || "Failed to load sessions");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    load();
    const interval = setInterval(load, 10000);
    return () => clearInterval(interval);
  }, [load]);

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
      <div className="mb-6">
        <h1 className="text-2xl font-bold">Active Sessions</h1>
        <p className="text-sm text-muted-foreground mt-1">
          {sessions.length} active session{sessions.length !== 1 ? "s" : ""}
        </p>
      </div>

      {error && (
        <ErrorAlert message={error} onRetry={load} onDismiss={() => setError("")} className="mb-4" />
      )}

      <div className="border rounded-lg overflow-hidden">
        <table className="w-full text-sm">
          <thead>
            <tr className="bg-muted/50 border-b">
              <th className="text-left p-3 font-medium hidden md:table-cell">Token ID</th>
              <th className="text-left p-3 font-medium">Type</th>
              <th className="text-left p-3 font-medium">Admin</th>
              <th className="text-left p-3 font-medium hidden md:table-cell">Created</th>
              <th className="text-left p-3 font-medium">Expires</th>
              <th className="text-right p-3 font-medium">Actions</th>
            </tr>
          </thead>
          <tbody>
            {sessions.length === 0 ? (
              <TableEmptyRow colSpan={6} message="No active sessions" />
            ) : (
              sessions.map((session) => (
                <tr
                  key={session.id}
                  className="border-b last:border-0 hover:bg-muted/30"
                >
                  <td className="p-3 font-mono text-xs hidden md:table-cell">
                    {session.id.substring(0, 12)}...
                  </td>
                  <td className="p-3">
                    <span className="inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium bg-blue-100 text-blue-700 dark:bg-blue-500/10 dark:text-blue-400">
                      {session.entity_type}
                    </span>
                  </td>
                  <td className="p-3">
                    <div>{session.name || "\u2014"}</div>
                    <div className="text-xs text-muted-foreground">
                      {session.email}
                    </div>
                  </td>
                  <td className="p-3 text-muted-foreground text-xs hidden md:table-cell">
                    <div>{formatTime(session.created_at)}</div>
                    <div>{relativeTime(session.created_at)}</div>
                  </td>
                  <td className="p-3 text-muted-foreground text-xs">
                    <div>{formatTime(session.expires_at)}</div>
                    <div>{relativeTime(session.expires_at)}</div>
                  </td>
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

      {/* Revoke Confirmation Dialog */}
      <Dialog
        open={!!revokeTarget}
        onClose={() => setRevokeTarget(null)}
      >
        <DialogHeader>
          <DialogTitle>Revoke Session</DialogTitle>
          <DialogCloseButton onClick={() => setRevokeTarget(null)} />
        </DialogHeader>

        <DialogBody>
          <p className="text-sm text-muted-foreground">
            Are you sure you want to revoke this session? The user will be logged out immediately.
          </p>
          {revokeTarget && (
            <div className="mt-3 p-3 bg-muted rounded-md text-sm space-y-1">
              <div><span className="font-medium">User:</span> {revokeTarget.name || revokeTarget.email}</div>
              <div><span className="font-medium">Type:</span> {revokeTarget.entity_type}</div>
              <div><span className="font-medium">Token:</span> <span className="font-mono text-xs">{revokeTarget.id.substring(0, 16)}...</span></div>
            </div>
          )}
        </DialogBody>

        <DialogFooter>
          <Button variant="outline" onClick={() => setRevokeTarget(null)}>
            Cancel
          </Button>
          <Button
            variant="destructive"
            disabled={acting === revokeTarget?.id}
            onClick={handleRevoke}
          >
            {acting === revokeTarget?.id ? "Revoking..." : "Revoke Session"}
          </Button>
        </DialogFooter>
      </Dialog>
    </div>
  );
}
