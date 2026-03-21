import { useEffect, useState } from "react";
import { get } from "@/lib/api";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import {
  Activity,
  AlertTriangle,
  Clock,
  Database,
  HardDrive,
  Layers,
  ListTodo,
  MemoryStick,
  Play,
  Shield,
  Timer,
  Users,
  UsersRound,
  KeySquare,
  ScrollText,
} from "lucide-react";
import { Link } from "react-router";
import { Spinner } from "@/components/ui/spinner";
import { ErrorAlert } from "@/components/ui/error-alert";

interface DashboardStats {
  system: {
    uptime_seconds: number;
    uptime: string;
    go_version: string;
    goroutines: number;
    memory_alloc_mb: number;
    memory_sys_mb: number;
  };
  database: {
    size_bytes: number;
    wal_size_bytes: number;
    tables: number;
    migrations: number;
  };
  queue: {
    pending: number;
    running: number;
    completed: number;
    failed: number;
    dead: number;
    cancelled: number;
  };
  cron: {
    total: number;
    enabled: number;
    running: number;
    next_run: string;
  };
  stats: {
    total_admins: number;
    total_users: number;
    active_sessions: number;
    active_api_keys: number;
  };
}

interface AuditEntry {
  id: number;
  admin_id: string;
  admin_email: string;
  admin_name: string;
  action: string;
  entity_type: string;
  entity_id: string;
  details: string;
  ip_address: string;
  created_at: string;
}

const ACTION_LABELS: Record<string, string> = {
  "admin.create": "Created admin",
  "admin.update": "Updated admin",
  "admin.delete": "Deleted admin",
  "user.create": "Created user",
  "user.update": "Updated user",
  "user.delete": "Deleted user",
  "user.impersonate": "Impersonated user",
  "session.revoke": "Revoked session",
  "setting.update": "Updated setting",
  "api_key.create": "Created API key",
  "api_key.update": "Updated API key",
  "api_key.revoke": "Revoked API key",
};

const ACTION_COLORS: Record<string, string> = {
  create: "bg-green-100 text-green-700",
  update: "bg-blue-100 text-blue-700",
  delete: "bg-red-100 text-red-700",
  revoke: "bg-orange-100 text-orange-700",
  impersonate: "bg-amber-100 text-amber-700",
};

function actionColor(action: string): string {
  const verb = action.split(".")[1] ?? "";
  return ACTION_COLORS[verb] ?? "bg-gray-100 text-gray-700";
}

function formatBytes(bytes: number): string {
  if (bytes === 0) return "0 B";
  const units = ["B", "KB", "MB", "GB"];
  const i = Math.floor(Math.log(bytes) / Math.log(1024));
  return `${(bytes / Math.pow(1024, i)).toFixed(1)} ${units[i]}`;
}

function formatRelativeTime(iso: string): string {
  const diff = (new Date(iso).getTime() - Date.now()) / 1000;
  if (diff < 0) return "now";
  if (diff < 60) return `${Math.round(diff)}s`;
  if (diff < 3600) return `in ${Math.round(diff / 60)}m`;
  return `in ${Math.round(diff / 3600)}h`;
}

function formatUptime(seconds: number): string {
  const d = Math.floor(seconds / 86400);
  const h = Math.floor((seconds % 86400) / 3600);
  const m = Math.floor((seconds % 3600) / 60);
  if (d > 0) return `${d}d ${h}h ${m}m`;
  if (h > 0) return `${h}h ${m}m`;
  return `${m}m`;
}

export default function DashboardPage() {
  const [stats, setStats] = useState<DashboardStats | null>(null);
  const [recentActivity, setRecentActivity] = useState<AuditEntry[]>([]);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;

    async function load() {
      try {
        const [data, activity] = await Promise.all([
          get<DashboardStats>("/admin/dashboard"),
          get<{ entries: AuditEntry[] }>("/admin/audit/recent"),
        ]);
        if (!cancelled) {
          setStats(data);
          setRecentActivity(activity.entries);
          setError(null);
        }
      } catch (err) {
        if (!cancelled) {
          setError(err instanceof Error ? err.message : "Failed to load stats");
        }
      }
    }

    load();
    const interval = setInterval(load, 30_000);
    return () => {
      cancelled = true;
      clearInterval(interval);
    };
  }, []);

  return (
    <div className="p-6">
      <div className="mb-6">
        <h1 className="text-2xl font-semibold tracking-tight">Dashboard</h1>
        <p className="text-sm text-muted-foreground">System overview and health metrics</p>
      </div>

      {error && (
        <ErrorAlert message={error} onRetry={() => { setError(null); }} className="mb-6" />
      )}

      {!stats && !error && <Spinner />}

      {stats && (
        <div className="space-y-6">
          <section>
            <h2 className="mb-3 text-sm font-medium text-muted-foreground uppercase tracking-wider">
              System
            </h2>
            <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
              <StatCard
                title="Uptime"
                value={formatUptime(stats.system.uptime_seconds)}
                icon={<Clock className="h-4 w-4" />}
              />
              <StatCard
                title="Memory"
                value={`${stats.system.memory_alloc_mb.toFixed(1)} MB`}
                description={`${stats.system.memory_sys_mb.toFixed(0)} MB reserved`}
                icon={<MemoryStick className="h-4 w-4" />}
              />
              <StatCard
                title="Goroutines"
                value={String(stats.system.goroutines)}
                icon={<Activity className="h-4 w-4" />}
              />
              <StatCard
                title="Go Version"
                value={stats.system.go_version.replace("go", "")}
                icon={<Layers className="h-4 w-4" />}
              />
            </div>
          </section>

          <section>
            <h2 className="mb-3 text-sm font-medium text-muted-foreground uppercase tracking-wider">
              Database
            </h2>
            <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
              <StatCard
                title="Database Size"
                value={formatBytes(stats.database.size_bytes)}
                icon={<HardDrive className="h-4 w-4" />}
              />
              <StatCard
                title="WAL Size"
                value={formatBytes(stats.database.wal_size_bytes)}
                icon={<Database className="h-4 w-4" />}
              />
              <StatCard
                title="Tables"
                value={String(stats.database.tables)}
                icon={<Layers className="h-4 w-4" />}
              />
              <StatCard
                title="Migrations"
                value={String(stats.database.migrations)}
                icon={<Database className="h-4 w-4" />}
              />
            </div>
          </section>

          <section>
            <h2 className="mb-3 text-sm font-medium text-muted-foreground uppercase tracking-wider">
              Job Queue
            </h2>
            <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
              <StatCard
                title="Pending"
                value={String(stats.queue.pending)}
                icon={<ListTodo className="h-4 w-4" />}
              />
              <StatCard
                title="Running"
                value={String(stats.queue.running)}
                icon={<Play className="h-4 w-4" />}
              />
              <StatCard
                title="Failed"
                value={String(stats.queue.failed)}
                description={stats.queue.dead > 0 ? `${stats.queue.dead} dead` : undefined}
                icon={<AlertTriangle className="h-4 w-4" />}
              />
              <StatCard
                title="Completed"
                value={String(stats.queue.completed)}
                icon={<Activity className="h-4 w-4" />}
              />
            </div>
          </section>

          <section>
            <h2 className="mb-3 text-sm font-medium text-muted-foreground uppercase tracking-wider">
              Cron Scheduler
            </h2>
            <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
              <StatCard
                title="Registered Jobs"
                value={String(stats.cron.total)}
                description={`${stats.cron.enabled} enabled`}
                icon={<Timer className="h-4 w-4" />}
              />
              <StatCard
                title="Running Now"
                value={String(stats.cron.running)}
                icon={<Play className="h-4 w-4" />}
              />
              <StatCard
                title="Next Run"
                value={stats.cron.next_run ? formatRelativeTime(stats.cron.next_run) : "—"}
                icon={<Clock className="h-4 w-4" />}
              />
            </div>
          </section>

          <section>
            <h2 className="mb-3 text-sm font-medium text-muted-foreground uppercase tracking-wider">
              Application
            </h2>
            <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
              <StatCard
                title="Admins"
                value={String(stats.stats.total_admins)}
                icon={<Shield className="h-4 w-4" />}
              />
              <StatCard
                title="Users"
                value={String(stats.stats.total_users)}
                icon={<UsersRound className="h-4 w-4" />}
              />
              <StatCard
                title="Active Sessions"
                value={String(stats.stats.active_sessions)}
                icon={<Users className="h-4 w-4" />}
              />
              <StatCard
                title="API Keys"
                value={String(stats.stats.active_api_keys)}
                icon={<KeySquare className="h-4 w-4" />}
              />
            </div>
          </section>

          {/* Recent Activity */}
          {recentActivity.length > 0 && (
            <section>
              <div className="flex items-center justify-between mb-3">
                <h2 className="text-sm font-medium text-muted-foreground uppercase tracking-wider">
                  Recent Activity
                </h2>
                <Link
                  to="/audit"
                  className="text-xs text-muted-foreground hover:text-foreground transition-colors"
                >
                  View all
                </Link>
              </div>
              <Card>
                <CardContent className="p-0">
                  <div className="divide-y">
                    {recentActivity.map((entry) => (
                      <div key={entry.id} className="flex items-center gap-3 px-4 py-2.5">
                        <ScrollText className="h-4 w-4 text-muted-foreground shrink-0" />
                        <div className="flex-1 min-w-0">
                          <div className="flex items-center gap-2">
                            <span
                              className={`inline-flex items-center px-1.5 py-0.5 rounded text-[10px] font-medium ${actionColor(entry.action)}`}
                            >
                              {ACTION_LABELS[entry.action] || entry.action}
                            </span>
                            {entry.entity_type && (
                              <span className="text-xs font-mono text-muted-foreground">
                                {entry.entity_type}
                                {entry.entity_id ? `#${entry.entity_id}` : ""}
                              </span>
                            )}
                          </div>
                          <div className="text-xs text-muted-foreground mt-0.5">
                            {entry.admin_name || entry.admin_email || `Admin #${entry.admin_id}`}
                            {entry.details ? ` \u2014 ${entry.details}` : ""}
                          </div>
                        </div>
                        <div className="text-xs text-muted-foreground shrink-0" title={formatRelativeTime(entry.created_at)}>
                          {formatRelativeTime(entry.created_at)}
                        </div>
                      </div>
                    ))}
                  </div>
                </CardContent>
              </Card>
            </section>
          )}
        </div>
      )}
    </div>
  );
}

function StatCard({
  title,
  value,
  description,
  icon,
}: {
  title: string;
  value: string;
  description?: string;
  icon: React.ReactNode;
}) {
  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between pb-2">
        <CardTitle className="text-sm font-medium">{title}</CardTitle>
        <span className="text-muted-foreground">{icon}</span>
      </CardHeader>
      <CardContent>
        <div className="text-2xl font-bold">{value}</div>
        {description && (
          <p className="text-xs text-muted-foreground mt-1">{description}</p>
        )}
      </CardContent>
    </Card>
  );
}
