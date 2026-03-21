import { useEffect, useState, useCallback } from "react";
import { get, post } from "@/lib/api";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import {
  RotateCw,
  Inbox,
  Play,
  CheckCircle,
  XCircle,
  Skull,
  Ban,
  RefreshCw,
} from "lucide-react";
import { Spinner } from "@/components/ui/spinner";
import { ErrorAlert } from "@/components/ui/error-alert";
import { EmptyState } from "@/components/ui/empty-state";

interface QueueStats {
  pending: number;
  running: number;
  completed: number;
  failed: number;
  dead: number;
  cancelled: number;
}

interface QueueJob {
  id: number;
  queue: string;
  type: string;
  payload: string;
  status: string;
  attempts: number;
  max_attempts: number;
  last_error: string;
  run_at: string;
  started_at: string;
  completed_at: string;
  created_at: string;
  updated_at: string;
}

interface JobsResponse {
  jobs: QueueJob[];
}

const STATUS_FILTERS = ["", "pending", "running", "completed", "failed", "dead", "cancelled"] as const;

function formatTime(iso: string): string {
  if (!iso) return "—";
  return new Date(iso).toLocaleString();
}

function StatusBadge({ status }: { status: string }) {
  const styles: Record<string, string> = {
    pending: "bg-yellow-100 text-yellow-700",
    running: "bg-blue-100 text-blue-700",
    completed: "bg-green-100 text-green-700",
    failed: "bg-red-100 text-red-700",
    dead: "bg-gray-100 text-gray-700",
    cancelled: "bg-gray-100 text-gray-500",
  };
  return (
    <span
      className={`inline-block rounded-full px-2 py-0.5 text-xs font-medium ${styles[status] || "bg-gray-100 text-gray-600"}`}
    >
      {status}
    </span>
  );
}

export default function QueuePage() {
  const [stats, setStats] = useState<QueueStats | null>(null);
  const [jobs, setJobs] = useState<QueueJob[]>([]);
  const [filter, setFilter] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [acting, setActing] = useState<number | null>(null);

  const loadStats = useCallback(async () => {
    try {
      const data = await get<QueueStats>("/admin/queue/stats");
      setStats(data);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load stats");
    }
  }, []);

  const loadJobs = useCallback(async () => {
    try {
      const params = new URLSearchParams({ limit: "50" });
      if (filter) params.set("status", filter);
      const data = await get<JobsResponse>(`/admin/queue/jobs?${params}`);
      setJobs(data.jobs);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load jobs");
    }
  }, [filter]);

  const loadAll = useCallback(async () => {
    setLoading(true);
    await Promise.all([loadStats(), loadJobs()]);
    setLoading(false);
  }, [loadStats, loadJobs]);

  useEffect(() => {
    loadAll();
    const interval = setInterval(loadAll, 10_000);
    return () => clearInterval(interval);
  }, [loadAll]);

  async function retryJob(id: number) {
    setActing(id);
    try {
      await post(`/admin/queue/jobs/${id}/retry`);
      await loadAll();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to retry job");
    } finally {
      setActing(null);
    }
  }

  async function cancelJob(id: number) {
    setActing(id);
    try {
      await post(`/admin/queue/jobs/${id}/cancel`);
      await loadAll();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to cancel job");
    } finally {
      setActing(null);
    }
  }

  const statCards = stats
    ? [
        { label: "Pending", value: stats.pending, icon: <Inbox className="h-4 w-4" /> },
        { label: "Running", value: stats.running, icon: <Play className="h-4 w-4" /> },
        { label: "Completed", value: stats.completed, icon: <CheckCircle className="h-4 w-4" /> },
        { label: "Failed", value: stats.failed, icon: <XCircle className="h-4 w-4" /> },
        { label: "Dead", value: stats.dead, icon: <Skull className="h-4 w-4" /> },
        { label: "Cancelled", value: stats.cancelled, icon: <Ban className="h-4 w-4" /> },
      ]
    : [];

  return (
    <div className="p-6">
      <div className="mb-6 flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight">Job Queue</h1>
          <p className="text-sm text-muted-foreground">
            Queue stats and job management
          </p>
        </div>
        <Button variant="outline" size="sm" onClick={loadAll} disabled={loading}>
          <RotateCw className={`h-4 w-4 ${loading ? "animate-spin" : ""}`} />
          Refresh
        </Button>
      </div>

      {error && (
        <ErrorAlert message={error} onRetry={loadAll} onDismiss={() => setError(null)} className="mb-6" />
      )}

      {stats && (
        <div className="mb-6 grid gap-4 sm:grid-cols-3 lg:grid-cols-6">
          {statCards.map((card) => (
            <Card key={card.label}>
              <CardHeader className="flex flex-row items-center justify-between pb-2">
                <CardTitle className="text-sm font-medium">{card.label}</CardTitle>
                <span className="text-muted-foreground">{card.icon}</span>
              </CardHeader>
              <CardContent>
                <div className="text-2xl font-bold">{card.value}</div>
              </CardContent>
            </Card>
          ))}
        </div>
      )}

      <Card>
        <CardHeader className="flex flex-row items-center justify-between">
          <CardTitle className="text-base">Jobs</CardTitle>
          <div className="flex items-center gap-1">
            {STATUS_FILTERS.map((s) => (
              <Button
                key={s || "all"}
                variant={filter === s ? "default" : "ghost"}
                size="sm"
                onClick={() => setFilter(s)}
              >
                {s || "All"}
              </Button>
            ))}
          </div>
        </CardHeader>
        <CardContent className="p-0">
          {loading && jobs.length === 0 ? (
            <div className="p-4"><Spinner /></div>
          ) : jobs.length === 0 ? (
            <EmptyState message="No jobs found" className="py-6" />
          ) : (
            <div className="overflow-x-auto">
              <table className="w-full text-sm">
                <thead>
                  <tr className="border-b border-border text-left text-muted-foreground">
                    <th className="px-4 py-3 font-medium">ID</th>
                    <th className="px-4 py-3 font-medium">Type</th>
                    <th className="px-4 py-3 font-medium">Status</th>
                    <th className="px-4 py-3 font-medium">Attempts</th>
                    <th className="px-4 py-3 font-medium">Created</th>
                    <th className="px-4 py-3 font-medium">Error</th>
                    <th className="px-4 py-3 font-medium text-right">Actions</th>
                  </tr>
                </thead>
                <tbody>
                  {jobs.map((job) => (
                    <tr key={job.id} className="border-b border-border last:border-0">
                      <td className="px-4 py-3 font-mono text-xs">{job.id}</td>
                      <td className="px-4 py-3 font-mono text-xs">{job.type}</td>
                      <td className="px-4 py-3">
                        <StatusBadge status={job.status} />
                      </td>
                      <td className="px-4 py-3 text-muted-foreground">
                        {job.attempts}/{job.max_attempts}
                      </td>
                      <td className="px-4 py-3 text-muted-foreground">
                        {formatTime(job.created_at)}
                      </td>
                      <td className="px-4 py-3 max-w-[200px] truncate text-destructive">
                        {job.last_error || "—"}
                      </td>
                      <td className="px-4 py-3">
                        <div className="flex items-center justify-end gap-1">
                          {(job.status === "failed" || job.status === "dead") && (
                            <Button
                              variant="ghost"
                              size="sm"
                              onClick={() => retryJob(job.id)}
                              disabled={acting !== null}
                              title="Retry"
                            >
                              <RefreshCw className="h-3 w-3" />
                            </Button>
                          )}
                          {job.status === "pending" && (
                            <Button
                              variant="ghost"
                              size="sm"
                              onClick={() => cancelJob(job.id)}
                              disabled={acting !== null}
                              title="Cancel"
                            >
                              <Ban className="h-3 w-3" />
                            </Button>
                          )}
                        </div>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
