import { useEffect, useState, useCallback } from "react";
import { toast } from "sonner";
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
  ChevronDown,
  ChevronRight,
  Search,
  X,
} from "lucide-react";
import { Skeleton } from "@/components/ui/skeleton";
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
  total: number;
}

const STATUS_FILTERS = ["", "pending", "running", "completed", "failed", "dead", "cancelled"] as const;
const PAGE_SIZE = 25;

function formatTime(iso: string): string {
  if (!iso) return "—";
  return new Date(iso).toLocaleString();
}

function formatDuration(startIso: string, endIso: string): string {
  if (!startIso || !endIso) return "—";
  const ms = new Date(endIso).getTime() - new Date(startIso).getTime();
  if (ms < 1000) return `${ms}ms`;
  const secs = ms / 1000;
  if (secs < 60) return `${secs.toFixed(1)}s`;
  const mins = Math.floor(secs / 60);
  const remSecs = Math.round(secs % 60);
  return `${mins}m ${remSecs}s`;
}

function formatPayload(raw: string): string {
  try {
    return JSON.stringify(JSON.parse(raw), null, 2);
  } catch {
    return raw;
  }
}

function StatusBadge({ status }: { status: string }) {
  const styles: Record<string, string> = {
    pending: "bg-yellow-100 text-yellow-700 dark:bg-yellow-500/10 dark:text-yellow-400",
    running: "bg-blue-100 text-blue-700 dark:bg-blue-500/10 dark:text-blue-400",
    completed: "bg-green-100 text-green-700 dark:bg-green-500/10 dark:text-green-400",
    failed: "bg-red-100 text-red-700 dark:bg-red-500/10 dark:text-red-400",
    dead: "bg-gray-100 text-gray-700 dark:bg-gray-500/10 dark:text-gray-400",
    cancelled: "bg-gray-100 text-gray-500 dark:bg-gray-500/10 dark:text-gray-400",
  };
  return (
    <span
      className={`inline-block rounded-full px-2 py-0.5 text-xs font-medium ${styles[status] || "bg-gray-100 text-gray-600 dark:bg-gray-500/10 dark:text-gray-400"}`}
    >
      {status}
    </span>
  );
}

function JobDetail({ job }: { job: QueueJob }) {
  return (
    <div className="grid gap-4 p-4 sm:grid-cols-2">
      <div>
        <h4 className="mb-2 text-xs font-medium uppercase text-muted-foreground">Details</h4>
        <dl className="space-y-1 text-sm">
          <div className="flex gap-2">
            <dt className="text-muted-foreground w-24 shrink-0">ID</dt>
            <dd className="font-mono">{job.id}</dd>
          </div>
          <div className="flex gap-2">
            <dt className="text-muted-foreground w-24 shrink-0">Queue</dt>
            <dd className="font-mono">{job.queue}</dd>
          </div>
          <div className="flex gap-2">
            <dt className="text-muted-foreground w-24 shrink-0">Type</dt>
            <dd className="font-mono">{job.type}</dd>
          </div>
          <div className="flex gap-2">
            <dt className="text-muted-foreground w-24 shrink-0">Status</dt>
            <dd><StatusBadge status={job.status} /></dd>
          </div>
          <div className="flex gap-2">
            <dt className="text-muted-foreground w-24 shrink-0">Attempts</dt>
            <dd>{job.attempts}/{job.max_attempts}</dd>
          </div>
        </dl>
      </div>
      <div>
        <h4 className="mb-2 text-xs font-medium uppercase text-muted-foreground">Timing</h4>
        <dl className="space-y-1 text-sm">
          <div className="flex gap-2">
            <dt className="text-muted-foreground w-24 shrink-0">Created</dt>
            <dd>{formatTime(job.created_at)}</dd>
          </div>
          <div className="flex gap-2">
            <dt className="text-muted-foreground w-24 shrink-0">Run at</dt>
            <dd>{formatTime(job.run_at)}</dd>
          </div>
          <div className="flex gap-2">
            <dt className="text-muted-foreground w-24 shrink-0">Started</dt>
            <dd>{formatTime(job.started_at)}</dd>
          </div>
          <div className="flex gap-2">
            <dt className="text-muted-foreground w-24 shrink-0">Completed</dt>
            <dd>{formatTime(job.completed_at)}</dd>
          </div>
          <div className="flex gap-2">
            <dt className="text-muted-foreground w-24 shrink-0">Duration</dt>
            <dd>{formatDuration(job.started_at, job.completed_at)}</dd>
          </div>
        </dl>
      </div>
      {job.payload && job.payload !== "{}" && (
        <div className="sm:col-span-2">
          <h4 className="mb-2 text-xs font-medium uppercase text-muted-foreground">Payload</h4>
          <pre className="rounded-md bg-muted p-3 text-xs font-mono overflow-x-auto max-h-48">
            {formatPayload(job.payload)}
          </pre>
        </div>
      )}
      {job.last_error && (
        <div className="sm:col-span-2">
          <h4 className="mb-2 text-xs font-medium uppercase text-muted-foreground">Last Error</h4>
          <pre className="rounded-md bg-destructive/10 p-3 text-xs font-mono text-destructive overflow-x-auto max-h-32">
            {job.last_error}
          </pre>
        </div>
      )}
    </div>
  );
}

export default function QueuePage() {
  const [stats, setStats] = useState<QueueStats | null>(null);
  const [jobs, setJobs] = useState<QueueJob[]>([]);
  const [total, setTotal] = useState(0);
  const [statusFilter, setStatusFilter] = useState("");
  const [typeFilter, setTypeFilter] = useState("");
  const [queueFilter, setQueueFilter] = useState("");
  const [offset, setOffset] = useState(0);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [acting, setActing] = useState<number | null>(null);
  const [expanded, setExpanded] = useState<number | null>(null);

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
      const params = new URLSearchParams({ limit: String(PAGE_SIZE), offset: String(offset) });
      if (statusFilter) params.set("status", statusFilter);
      if (typeFilter) params.set("type", typeFilter);
      if (queueFilter) params.set("queue", queueFilter);
      const data = await get<JobsResponse>(`/admin/queue/jobs?${params}`);
      setJobs(data.jobs);
      setTotal(data.total);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load jobs");
    }
  }, [statusFilter, typeFilter, queueFilter, offset]);

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

  // Reset to first page when filters change.
  function applyStatusFilter(s: string) {
    setStatusFilter(s);
    setOffset(0);
    setExpanded(null);
  }

  function applyTypeFilter(val: string) {
    setTypeFilter(val);
    setOffset(0);
    setExpanded(null);
  }

  function applyQueueFilter(val: string) {
    setQueueFilter(val);
    setOffset(0);
    setExpanded(null);
  }

  async function retryJob(id: number) {
    setActing(id);
    try {
      await post(`/admin/queue/jobs/${id}/retry`);
      toast.success("Job queued for retry");
      await loadAll();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Failed to retry job");
    } finally {
      setActing(null);
    }
  }

  async function cancelJob(id: number) {
    setActing(id);
    try {
      await post(`/admin/queue/jobs/${id}/cancel`);
      toast.success("Job cancelled");
      await loadAll();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Failed to cancel job");
    } finally {
      setActing(null);
    }
  }

  function toggleExpand(id: number) {
    setExpanded((prev) => (prev === id ? null : id));
  }

  const hasMore = offset + PAGE_SIZE < total;
  const hasPrev = offset > 0;

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

  const hasActiveFilters = typeFilter !== "" || queueFilter !== "";

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
        <CardHeader className="flex flex-col gap-3">
          <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
            <CardTitle className="text-base">Jobs</CardTitle>
            <div className="flex flex-wrap items-center gap-1">
              {STATUS_FILTERS.map((s) => (
                <Button
                  key={s || "all"}
                  variant={statusFilter === s ? "default" : "ghost"}
                  size="sm"
                  onClick={() => applyStatusFilter(s)}
                >
                  {s || "All"}
                </Button>
              ))}
            </div>
          </div>
          <div className="flex flex-wrap items-center gap-2">
            <div className="relative">
              <Search className="absolute left-2.5 top-2.5 h-3.5 w-3.5 text-muted-foreground" />
              <input
                type="text"
                placeholder="Filter by type..."
                value={typeFilter}
                onChange={(e) => applyTypeFilter(e.target.value)}
                className="h-8 rounded-md border border-input bg-background pl-8 pr-8 text-sm placeholder:text-muted-foreground focus:outline-none focus:ring-1 focus:ring-ring w-40"
              />
              {typeFilter && (
                <button
                  onClick={() => applyTypeFilter("")}
                  className="absolute right-2 top-2.5 text-muted-foreground hover:text-foreground"
                >
                  <X className="h-3.5 w-3.5" />
                </button>
              )}
            </div>
            <div className="relative">
              <Search className="absolute left-2.5 top-2.5 h-3.5 w-3.5 text-muted-foreground" />
              <input
                type="text"
                placeholder="Filter by queue..."
                value={queueFilter}
                onChange={(e) => applyQueueFilter(e.target.value)}
                className="h-8 rounded-md border border-input bg-background pl-8 pr-8 text-sm placeholder:text-muted-foreground focus:outline-none focus:ring-1 focus:ring-ring w-40"
              />
              {queueFilter && (
                <button
                  onClick={() => applyQueueFilter("")}
                  className="absolute right-2 top-2.5 text-muted-foreground hover:text-foreground"
                >
                  <X className="h-3.5 w-3.5" />
                </button>
              )}
            </div>
            {hasActiveFilters && (
              <Button
                variant="ghost"
                size="sm"
                onClick={() => { applyTypeFilter(""); applyQueueFilter(""); }}
                className="text-xs"
              >
                Clear filters
              </Button>
            )}
          </div>
        </CardHeader>
        <CardContent className="p-0">
          {loading && jobs.length === 0 ? (
            <div className="p-4 space-y-3">
              {Array.from({ length: 3 }, (_, i) => (
                <div key={i} className="flex items-center gap-4">
                  <Skeleton className="h-4 w-4" />
                  <Skeleton className="h-4 w-16" />
                  <Skeleton className="h-4 w-24" />
                  <Skeleton className="h-4 w-20" />
                  <Skeleton className="h-4 w-28 hidden md:block" />
                </div>
              ))}
            </div>
          ) : jobs.length === 0 ? (
            <EmptyState message="No jobs found" className="py-6" />
          ) : (
            <>
              <div className="overflow-x-auto">
                <table className="w-full text-sm">
                  <thead>
                    <tr className="border-b border-border text-left text-muted-foreground">
                      <th className="px-4 py-3 font-medium w-8"></th>
                      <th className="px-4 py-3 font-medium hidden md:table-cell">ID</th>
                      <th className="px-4 py-3 font-medium">Type</th>
                      <th className="px-4 py-3 font-medium">Status</th>
                      <th className="px-4 py-3 font-medium hidden md:table-cell">Attempts</th>
                      <th className="px-4 py-3 font-medium hidden md:table-cell">Created</th>
                      <th className="px-4 py-3 font-medium hidden lg:table-cell">Error</th>
                      <th className="px-4 py-3 font-medium text-right">Actions</th>
                    </tr>
                  </thead>
                  <tbody>
                    {jobs.map((job) => (
                      <>
                        <tr
                          key={job.id}
                          className={`border-b border-border cursor-pointer hover:bg-muted/50 ${
                            expanded === job.id ? "bg-muted/30" : ""
                          }`}
                          onClick={() => toggleExpand(job.id)}
                        >
                          <td className="px-4 py-3 text-muted-foreground">
                            {expanded === job.id ? (
                              <ChevronDown className="h-4 w-4" />
                            ) : (
                              <ChevronRight className="h-4 w-4" />
                            )}
                          </td>
                          <td className="px-4 py-3 font-mono text-xs hidden md:table-cell">{job.id}</td>
                          <td className="px-4 py-3 font-mono text-xs">{job.type}</td>
                          <td className="px-4 py-3">
                            <StatusBadge status={job.status} />
                          </td>
                          <td className="px-4 py-3 text-muted-foreground hidden md:table-cell">
                            {job.attempts}/{job.max_attempts}
                          </td>
                          <td className="px-4 py-3 text-muted-foreground hidden md:table-cell">
                            {formatTime(job.created_at)}
                          </td>
                          <td className="px-4 py-3 max-w-[200px] truncate text-destructive hidden lg:table-cell">
                            {job.last_error || "—"}
                          </td>
                          <td className="px-4 py-3">
                            <div className="flex items-center justify-end gap-1" onClick={(e) => e.stopPropagation()}>
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
                        {expanded === job.id && (
                          <tr key={`${job.id}-detail`} className="border-b border-border">
                            <td colSpan={8} className="bg-muted/20 p-0">
                              <JobDetail job={job} />
                            </td>
                          </tr>
                        )}
                      </>
                    ))}
                  </tbody>
                </table>
              </div>
              {(hasPrev || hasMore) && (
                <div className="flex items-center justify-between border-t border-border px-4 py-3">
                  <span className="text-sm text-muted-foreground">
                    {offset + 1}–{Math.min(offset + PAGE_SIZE, total)} of {total}
                  </span>
                  <div className="flex gap-1">
                    <Button
                      variant="ghost"
                      size="sm"
                      onClick={() => { setOffset(Math.max(0, offset - PAGE_SIZE)); setExpanded(null); }}
                      disabled={!hasPrev}
                    >
                      Previous
                    </Button>
                    <Button
                      variant="ghost"
                      size="sm"
                      onClick={() => { setOffset(offset + PAGE_SIZE); setExpanded(null); }}
                      disabled={!hasMore}
                    >
                      Next
                    </Button>
                  </div>
                </div>
              )}
            </>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
