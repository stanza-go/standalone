import { useCallback, useEffect, useState } from "react";
import { useParams } from "react-router";
import { toast } from "sonner";
import { get, del } from "@/lib/api";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Spinner } from "@/components/ui/spinner";
import { ErrorAlert } from "@/components/ui/error-alert";
import { TableEmptyRow } from "@/components/ui/empty-state";
import { Pagination } from "@/components/ui/pagination";
import {
  Shield,
  Activity,
  Monitor,
  Trash2,
} from "lucide-react";
import { Breadcrumb } from "@/components/ui/breadcrumb";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";

interface AdminDetail {
  id: number;
  email: string;
  name: string;
  role: string;
  is_active: boolean;
  created_at: string;
  updated_at: string;
}

interface ActivityEntry {
  id: number;
  action: string;
  entity_type: string;
  entity_id: string;
  details: string;
  ip_address: string;
  created_at: string;
}

interface Session {
  id: string;
  created_at: string;
  expires_at: string;
}

type Tab = "activity" | "sessions";

export default function AdminDetailPage() {
  const { id } = useParams<{ id: string }>();
  const [admin, setAdmin] = useState<AdminDetail | null>(null);
  const [sessionCount, setSessionCount] = useState(0);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  const [tab, setTab] = useState<Tab>("activity");

  // Activity
  const [activity, setActivity] = useState<ActivityEntry[]>([]);
  const [activityTotal, setActivityTotal] = useState(0);
  const [activityPage, setActivityPage] = useState(0);
  const [activityLoading, setActivityLoading] = useState(false);

  // Sessions
  const [sessions, setSessions] = useState<Session[]>([]);
  const [sessionsLoading, setSessionsLoading] = useState(false);
  const [revokingSession, setRevokingSession] = useState<string | null>(null);
  const [revokeTarget, setRevokeTarget] = useState<Session | null>(null);

  const pageSize = 20;

  const loadAdmin = useCallback(async () => {
    try {
      const data = await get<{ admin: AdminDetail; active_sessions: number }>(
        `/admin/admins/${id}`
      );
      setAdmin(data.admin);
      setSessionCount(data.active_sessions);
      setError("");
    } catch (e: any) {
      setError(e.message || "Failed to load admin");
    } finally {
      setLoading(false);
    }
  }, [id]);

  const loadActivity = useCallback(async () => {
    setActivityLoading(true);
    try {
      const params = new URLSearchParams();
      params.set("limit", String(pageSize));
      params.set("offset", String(activityPage * pageSize));
      const data = await get<{ entries: ActivityEntry[]; total: number }>(
        `/admin/admins/${id}/activity?${params}`
      );
      setActivity(data.entries);
      setActivityTotal(data.total);
    } catch {
      // Non-critical
    } finally {
      setActivityLoading(false);
    }
  }, [id, activityPage]);

  const loadSessions = useCallback(async () => {
    setSessionsLoading(true);
    try {
      const data = await get<{ sessions: Session[]; total: number }>(
        `/admin/admins/${id}/sessions`
      );
      setSessions(data.sessions);
      setSessionCount(data.total);
    } catch {
      // Non-critical
    } finally {
      setSessionsLoading(false);
    }
  }, [id]);

  useEffect(() => {
    loadAdmin();
  }, [loadAdmin]);

  useEffect(() => {
    if (tab === "activity") loadActivity();
  }, [tab, loadActivity]);

  useEffect(() => {
    if (tab === "sessions") loadSessions();
  }, [tab, loadSessions]);

  async function revokeSession() {
    if (!revokeTarget) return;
    const sessionId = revokeTarget.id;
    setRevokingSession(sessionId);
    try {
      await del(`/admin/sessions/${sessionId}`);
      setRevokeTarget(null);
      toast.success("Session revoked");
      loadSessions();
    } catch (e: any) {
      toast.error(e.message || "Failed to revoke session");
    } finally {
      setRevokingSession(null);
    }
  }

  function formatTime(iso: string): string {
    if (!iso) return "\u2014";
    const d = new Date(iso);
    return d.toLocaleDateString() + " " + d.toLocaleTimeString();
  }

  function relativeTime(iso: string): string {
    if (!iso) return "\u2014";
    const now = Date.now();
    const then = new Date(iso).getTime();
    const diff = now - then;
    const mins = Math.floor(diff / 60000);
    if (mins < 1) return "just now";
    if (mins < 60) return `${mins}m ago`;
    const hours = Math.floor(mins / 60);
    if (hours < 24) return `${hours}h ago`;
    const days = Math.floor(hours / 24);
    return `${days}d ago`;
  }

  function actionLabel(action: string): string {
    const parts = action.split(".");
    if (parts.length === 2) {
      const entity = parts[0].charAt(0).toUpperCase() + parts[0].slice(1);
      const verb = parts[1].charAt(0).toUpperCase() + parts[1].slice(1);
      return `${entity} ${verb}`;
    }
    return action;
  }

  if (loading) {
    return (
      <div className="p-6">
        <Spinner />
      </div>
    );
  }

  if (error || !admin) {
    return (
      <div className="p-6">
        <Breadcrumb items={[{ label: "Admin Users", to: "/admins" }, { label: "Not Found" }]} className="mb-4" />
        <ErrorAlert message={error || "Admin not found"} onRetry={loadAdmin} />
      </div>
    );
  }

  const tabs: { key: Tab; label: string; icon: React.ReactNode; count?: number }[] = [
    { key: "activity", label: "Activity", icon: <Activity className="h-4 w-4" />, count: activityTotal },
    { key: "sessions", label: "Sessions", icon: <Monitor className="h-4 w-4" />, count: sessionCount },
  ];

  const activityPages = Math.ceil(activityTotal / pageSize);

  return (
    <div className="p-6">
      {/* Breadcrumb + Header */}
      <Breadcrumb items={[{ label: "Admin Users", to: "/admins" }, { label: admin.email }]} className="mb-2" />
      <h1 className="text-2xl font-bold mb-6">Admin Detail</h1>

      {/* Profile card */}
      <Card className="mb-6">
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <Shield className="h-5 w-5" />
            Profile
          </CardTitle>
        </CardHeader>
        <CardContent>
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
            <div>
              <p className="text-xs text-muted-foreground mb-1">Email</p>
              <p className="font-medium">{admin.email}</p>
            </div>
            <div>
              <p className="text-xs text-muted-foreground mb-1">Name</p>
              <p className="font-medium">{admin.name || "\u2014"}</p>
            </div>
            <div>
              <p className="text-xs text-muted-foreground mb-1">Role</p>
              <span
                className={`inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium ${
                  admin.role === "superadmin"
                    ? "bg-purple-100 text-purple-700 dark:bg-purple-500/10 dark:text-purple-400"
                    : admin.role === "admin"
                      ? "bg-blue-100 text-blue-700 dark:bg-blue-500/10 dark:text-blue-400"
                      : "bg-gray-100 text-gray-700 dark:bg-gray-500/10 dark:text-gray-400"
                }`}
              >
                {admin.role}
              </span>
            </div>
            <div>
              <p className="text-xs text-muted-foreground mb-1">Status</p>
              <span
                className={`inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium ${
                  admin.is_active ? "bg-green-100 text-green-700 dark:bg-green-500/10 dark:text-green-400" : "bg-red-100 text-red-700 dark:bg-red-500/10 dark:text-red-400"
                }`}
              >
                {admin.is_active ? "Active" : "Inactive"}
              </span>
            </div>
            <div>
              <p className="text-xs text-muted-foreground mb-1">Created</p>
              <p className="text-sm">{formatTime(admin.created_at)}</p>
            </div>
            <div>
              <p className="text-xs text-muted-foreground mb-1">Active Sessions</p>
              <p className="text-sm font-medium">{sessionCount}</p>
            </div>
          </div>
        </CardContent>
      </Card>

      {/* Tabs */}
      <div className="flex border-b mb-4">
        {tabs.map((t) => (
          <button
            key={t.key}
            onClick={() => setTab(t.key)}
            className={`flex items-center gap-2 px-4 py-2 text-sm font-medium border-b-2 transition-colors ${
              tab === t.key
                ? "border-primary text-foreground"
                : "border-transparent text-muted-foreground hover:text-foreground"
            }`}
          >
            {t.icon}
            {t.label}
            {t.count !== undefined && t.count > 0 && (
              <span className="ml-1 text-xs bg-muted px-1.5 py-0.5 rounded-full">
                {t.count}
              </span>
            )}
          </button>
        ))}
      </div>

      {/* Activity tab */}
      {tab === "activity" && (
        <div>
          {activityLoading ? (
            <Spinner />
          ) : (
            <>
              <div className="border rounded-lg overflow-hidden">
                <table className="w-full text-sm">
                  <thead>
                    <tr className="bg-muted/50 border-b">
                      <th className="text-left p-3 font-medium">Action</th>
                      <th className="text-left p-3 font-medium hidden md:table-cell">Target</th>
                      <th className="text-left p-3 font-medium hidden lg:table-cell">Details</th>
                      <th className="text-left p-3 font-medium hidden md:table-cell">IP</th>
                      <th className="text-left p-3 font-medium">Time</th>
                    </tr>
                  </thead>
                  <tbody>
                    {activity.length === 0 ? (
                      <TableEmptyRow colSpan={5} message="No activity recorded" />
                    ) : (
                      activity.map((e) => (
                        <tr key={e.id} className="border-b last:border-0 hover:bg-muted/30">
                          <td className="p-3">
                            <span className="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-muted">
                              {actionLabel(e.action)}
                            </span>
                          </td>
                          <td className="p-3 hidden md:table-cell">
                            <span className="text-xs text-muted-foreground">
                              {e.entity_type} #{e.entity_id}
                            </span>
                          </td>
                          <td className="p-3 hidden lg:table-cell text-muted-foreground text-xs max-w-[200px] truncate">
                            {e.details || "\u2014"}
                          </td>
                          <td className="p-3 hidden md:table-cell font-mono text-xs text-muted-foreground">
                            {e.ip_address}
                          </td>
                          <td className="p-3 text-xs text-muted-foreground" title={formatTime(e.created_at)}>
                            {relativeTime(e.created_at)}
                          </td>
                        </tr>
                      ))
                    )}
                  </tbody>
                </table>
              </div>
              <Pagination
                page={activityPage}
                totalPages={activityPages}
                total={activityTotal}
                pageSize={pageSize}
                onPrev={() => setActivityPage(activityPage - 1)}
                onNext={() => setActivityPage(activityPage + 1)}
              />
            </>
          )}
        </div>
      )}

      {/* Sessions tab */}
      {tab === "sessions" && (
        <div>
          {sessionsLoading ? (
            <Spinner />
          ) : (
            <div className="border rounded-lg overflow-hidden">
              <table className="w-full text-sm">
                <thead>
                  <tr className="bg-muted/50 border-b">
                    <th className="text-left p-3 font-medium">Session ID</th>
                    <th className="text-left p-3 font-medium hidden md:table-cell">Created</th>
                    <th className="text-left p-3 font-medium">Expires</th>
                    <th className="text-right p-3 font-medium">Actions</th>
                  </tr>
                </thead>
                <tbody>
                  {sessions.length === 0 ? (
                    <TableEmptyRow colSpan={4} message="No active sessions" />
                  ) : (
                    sessions.map((s) => (
                      <tr key={s.id} className="border-b last:border-0 hover:bg-muted/30">
                        <td className="p-3 font-mono text-xs">{s.id.substring(0, 12)}...</td>
                        <td className="p-3 text-xs text-muted-foreground hidden md:table-cell">
                          {relativeTime(s.created_at)}
                        </td>
                        <td className="p-3 text-xs text-muted-foreground">
                          {formatTime(s.expires_at)}
                        </td>
                        <td className="p-3 text-right">
                          <Button
                            variant="ghost"
                            size="sm"
                            onClick={() => setRevokeTarget(s)}
                            disabled={revokingSession === s.id}
                          >
                            <Trash2 className="h-3.5 w-3.5 text-red-500" />
                          </Button>
                        </td>
                      </tr>
                    ))
                  )}
                </tbody>
              </table>
            </div>
          )}
        </div>
      )}

      {/* Revoke Session Confirmation */}
      <ConfirmDialog
        open={!!revokeTarget}
        onClose={() => setRevokeTarget(null)}
        onConfirm={revokeSession}
        title="Revoke Session"
        message="Are you sure you want to revoke this session? The admin will be logged out immediately."
        confirmLabel="Revoke"
        loading={revokingSession === revokeTarget?.id}
        details={revokeTarget && (
          <>
            <div><span className="font-medium">Token:</span> <span className="font-mono text-xs">{revokeTarget.id.substring(0, 16)}...</span></div>
            <div><span className="font-medium">Expires:</span> {formatTime(revokeTarget.expires_at)}</div>
          </>
        )}
      />
    </div>
  );
}

