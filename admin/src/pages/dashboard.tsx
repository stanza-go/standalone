import { useEffect, useState } from "react";
import { get } from "@/lib/api";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import {
  Activity,
  Clock,
  Database,
  HardDrive,
  MemoryStick,
  Users,
  Shield,
  Layers,
} from "lucide-react";

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
  stats: {
    total_admins: number;
    active_sessions: number;
  };
}

function formatBytes(bytes: number): string {
  if (bytes === 0) return "0 B";
  const units = ["B", "KB", "MB", "GB"];
  const i = Math.floor(Math.log(bytes) / Math.log(1024));
  return `${(bytes / Math.pow(1024, i)).toFixed(1)} ${units[i]}`;
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
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;

    async function load() {
      try {
        const data = await get<DashboardStats>("/admin/dashboard");
        if (!cancelled) {
          setStats(data);
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
        <div className="mb-6 rounded-md border border-destructive/50 bg-destructive/5 p-3 text-sm text-destructive">
          {error}
        </div>
      )}

      {!stats && !error && (
        <div className="text-sm text-muted-foreground">Loading...</div>
      )}

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
              Application
            </h2>
            <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
              <StatCard
                title="Admins"
                value={String(stats.stats.total_admins)}
                icon={<Shield className="h-4 w-4" />}
              />
              <StatCard
                title="Active Sessions"
                value={String(stats.stats.active_sessions)}
                icon={<Users className="h-4 w-4" />}
              />
            </div>
          </section>
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
