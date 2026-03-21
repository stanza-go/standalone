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
  User,
  Activity,
  Monitor,
  Upload,
  Trash2,
  Image,
  FileText,
  Film,
  File,
} from "lucide-react";
import { Breadcrumb } from "@/components/ui/breadcrumb";

interface UserDetail {
  id: number;
  email: string;
  name: string;
  is_active: boolean;
  created_at: string;
  updated_at: string;
}

interface ActivityEntry {
  id: number;
  admin_id: string;
  admin_email: string;
  admin_name: string;
  action: string;
  details: string;
  ip_address: string;
  created_at: string;
}

interface Session {
  id: string;
  created_at: string;
  expires_at: string;
}

interface UploadEntry {
  id: number;
  uuid: string;
  original_name: string;
  content_type: string;
  size_bytes: number;
  has_thumbnail: boolean;
  created_at: string;
}

type Tab = "activity" | "sessions" | "uploads";

export default function UserDetailPage() {
  const { id } = useParams<{ id: string }>();
  const [user, setUser] = useState<UserDetail | null>(null);
  const [sessionCount, setSessionCount] = useState(0);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  // Tabs
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

  // Uploads
  const [uploads, setUploads] = useState<UploadEntry[]>([]);
  const [uploadsTotal, setUploadsTotal] = useState(0);
  const [uploadsPage, setUploadsPage] = useState(0);
  const [uploadsLoading, setUploadsLoading] = useState(false);

  const pageSize = 20;

  const loadUser = useCallback(async () => {
    try {
      const data = await get<{ user: UserDetail; active_sessions: number }>(
        `/admin/users/${id}`
      );
      setUser(data.user);
      setSessionCount(data.active_sessions);
      setError("");
    } catch (e: any) {
      setError(e.message || "Failed to load user");
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
        `/admin/users/${id}/activity?${params}`
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
        `/admin/users/${id}/sessions`
      );
      setSessions(data.sessions);
      setSessionCount(data.total);
    } catch {
      // Non-critical
    } finally {
      setSessionsLoading(false);
    }
  }, [id]);

  const loadUploads = useCallback(async () => {
    setUploadsLoading(true);
    try {
      const params = new URLSearchParams();
      params.set("limit", String(pageSize));
      params.set("offset", String(uploadsPage * pageSize));
      const data = await get<{ uploads: UploadEntry[]; total: number }>(
        `/admin/users/${id}/uploads?${params}`
      );
      setUploads(data.uploads);
      setUploadsTotal(data.total);
    } catch {
      // Non-critical
    } finally {
      setUploadsLoading(false);
    }
  }, [id, uploadsPage]);

  useEffect(() => {
    loadUser();
  }, [loadUser]);

  useEffect(() => {
    if (tab === "activity") loadActivity();
  }, [tab, loadActivity]);

  useEffect(() => {
    if (tab === "sessions") loadSessions();
  }, [tab, loadSessions]);

  useEffect(() => {
    if (tab === "uploads") loadUploads();
  }, [tab, loadUploads]);

  async function revokeSession(sessionId: string) {
    setRevokingSession(sessionId);
    try {
      await del(`/admin/sessions/${sessionId}`);
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

  function formatSize(bytes: number): string {
    if (bytes < 1024) return `${bytes} B`;
    if (bytes < 1048576) return `${(bytes / 1024).toFixed(1)} KB`;
    return `${(bytes / 1048576).toFixed(1)} MB`;
  }

  function fileIcon(contentType: string) {
    if (contentType.startsWith("image/")) return <Image className="h-4 w-4 text-blue-500" />;
    if (contentType.startsWith("video/")) return <Film className="h-4 w-4 text-purple-500" />;
    if (contentType === "application/pdf") return <FileText className="h-4 w-4 text-red-500" />;
    return <File className="h-4 w-4 text-muted-foreground" />;
  }

  function actionLabel(action: string): string {
    const map: Record<string, string> = {
      "user.create": "Created",
      "user.update": "Updated",
      "user.delete": "Deleted",
      "user.impersonate": "Impersonated",
    };
    return map[action] || action;
  }

  if (loading) {
    return (
      <div className="p-6">
        <Spinner />
      </div>
    );
  }

  if (error || !user) {
    return (
      <div className="p-6">
        <Breadcrumb items={[{ label: "Users", to: "/users" }, { label: "Not Found" }]} className="mb-4" />
        <ErrorAlert message={error || "User not found"} onRetry={loadUser} />
      </div>
    );
  }

  const tabs: { key: Tab; label: string; icon: React.ReactNode; count?: number }[] = [
    { key: "activity", label: "Activity", icon: <Activity className="h-4 w-4" />, count: activityTotal },
    { key: "sessions", label: "Sessions", icon: <Monitor className="h-4 w-4" />, count: sessionCount },
    { key: "uploads", label: "Uploads", icon: <Upload className="h-4 w-4" />, count: uploadsTotal },
  ];

  const activityPages = Math.ceil(activityTotal / pageSize);
  const uploadsPages = Math.ceil(uploadsTotal / pageSize);

  return (
    <div className="p-6">
      {/* Breadcrumb + Header */}
      <Breadcrumb items={[{ label: "Users", to: "/users" }, { label: user.email }]} className="mb-2" />
      <h1 className="text-2xl font-bold mb-6">User Detail</h1>

      {/* Profile card */}
      <Card className="mb-6">
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <User className="h-5 w-5" />
            Profile
          </CardTitle>
        </CardHeader>
        <CardContent>
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
            <div>
              <p className="text-xs text-muted-foreground mb-1">Email</p>
              <p className="font-medium">{user.email}</p>
            </div>
            <div>
              <p className="text-xs text-muted-foreground mb-1">Name</p>
              <p className="font-medium">{user.name || "\u2014"}</p>
            </div>
            <div>
              <p className="text-xs text-muted-foreground mb-1">Status</p>
              <span
                className={`inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium ${
                  user.is_active ? "bg-green-100 text-green-700" : "bg-red-100 text-red-700"
                }`}
              >
                {user.is_active ? "Active" : "Inactive"}
              </span>
            </div>
            <div>
              <p className="text-xs text-muted-foreground mb-1">Created</p>
              <p className="text-sm">{formatTime(user.created_at)}</p>
            </div>
            <div>
              <p className="text-xs text-muted-foreground mb-1">Updated</p>
              <p className="text-sm">{formatTime(user.updated_at)}</p>
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

      {/* Tab content */}
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
                      <th className="text-left p-3 font-medium hidden md:table-cell">By</th>
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
                            <span className="text-xs">{e.admin_email || e.admin_id}</span>
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
                            onClick={() => revokeSession(s.id)}
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

      {tab === "uploads" && (
        <div>
          {uploadsLoading ? (
            <Spinner />
          ) : (
            <>
              <div className="border rounded-lg overflow-hidden">
                <table className="w-full text-sm">
                  <thead>
                    <tr className="bg-muted/50 border-b">
                      <th className="text-left p-3 font-medium w-8"></th>
                      <th className="text-left p-3 font-medium">Name</th>
                      <th className="text-left p-3 font-medium hidden md:table-cell">Type</th>
                      <th className="text-left p-3 font-medium hidden md:table-cell">Size</th>
                      <th className="text-left p-3 font-medium">Uploaded</th>
                    </tr>
                  </thead>
                  <tbody>
                    {uploads.length === 0 ? (
                      <TableEmptyRow colSpan={5} message="No uploads" />
                    ) : (
                      uploads.map((u) => (
                        <tr key={u.id} className="border-b last:border-0 hover:bg-muted/30">
                          <td className="p-3">
                            {u.has_thumbnail ? (
                              <img
                                src={`/api/admin/uploads/${u.id}/thumb`}
                                alt=""
                                className="h-8 w-8 rounded object-cover"
                              />
                            ) : (
                              fileIcon(u.content_type)
                            )}
                          </td>
                          <td className="p-3">
                            <a
                              href={`/api/admin/uploads/${u.id}/file`}
                              target="_blank"
                              rel="noopener noreferrer"
                              className="hover:underline text-sm"
                            >
                              {u.original_name}
                            </a>
                          </td>
                          <td className="p-3 text-xs text-muted-foreground hidden md:table-cell">
                            {u.content_type}
                          </td>
                          <td className="p-3 text-xs text-muted-foreground hidden md:table-cell">
                            {formatSize(u.size_bytes)}
                          </td>
                          <td className="p-3 text-xs text-muted-foreground" title={formatTime(u.created_at)}>
                            {relativeTime(u.created_at)}
                          </td>
                        </tr>
                      ))
                    )}
                  </tbody>
                </table>
              </div>
              <Pagination
                page={uploadsPage}
                totalPages={uploadsPages}
                total={uploadsTotal}
                pageSize={pageSize}
                onPrev={() => setUploadsPage(uploadsPage - 1)}
                onNext={() => setUploadsPage(uploadsPage + 1)}
              />
            </>
          )}
        </div>
      )}
    </div>
  );
}

