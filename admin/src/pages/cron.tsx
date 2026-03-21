import { useEffect, useState, useCallback } from "react";
import { toast } from "sonner";
import { get, post } from "@/lib/api";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import {
  Clock,
  Play,
  Pause,
  RotateCw,
  CheckCircle,
  XCircle,
  Loader2,
  History,
  ChevronDown,
  ChevronRight,
} from "lucide-react";
import { Spinner } from "@/components/ui/spinner";
import { ErrorAlert } from "@/components/ui/error-alert";
import { EmptyState } from "@/components/ui/empty-state";

interface CronEntry {
  name: string;
  schedule: string;
  enabled: boolean;
  running: boolean;
  last_run: string;
  next_run: string;
  last_err: string;
}

interface CronResponse {
  entries: CronEntry[];
}

interface CronRun {
  id: number;
  name: string;
  started_at: string;
  duration_ms: number;
  status: string;
  error: string;
}

interface RunsResponse {
  runs: CronRun[];
  total: number;
}

function formatTime(iso: string): string {
  if (!iso) return "—";
  const d = new Date(iso);
  return d.toLocaleString();
}

function relativeTime(iso: string): string {
  if (!iso) return "";
  const now = Date.now();
  const t = new Date(iso).getTime();
  const diff = t - now;
  const abs = Math.abs(diff);
  if (abs < 60_000) return diff > 0 ? "in <1m" : "<1m ago";
  const mins = Math.round(abs / 60_000);
  if (mins < 60) return diff > 0 ? `in ${mins}m` : `${mins}m ago`;
  const hrs = Math.round(mins / 60);
  return diff > 0 ? `in ${hrs}h` : `${hrs}h ago`;
}

function formatDuration(ms: number): string {
  if (ms < 1000) return `${ms}ms`;
  const secs = ms / 1000;
  if (secs < 60) return `${secs.toFixed(1)}s`;
  const mins = Math.floor(secs / 60);
  const remSecs = Math.round(secs % 60);
  return `${mins}m ${remSecs}s`;
}

function RunHistory({ name }: { name: string }) {
  const [runs, setRuns] = useState<CronRun[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [limit] = useState(20);
  const [offset, setOffset] = useState(0);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const data = await get<RunsResponse>(
        `/admin/cron/${encodeURIComponent(name)}/runs?limit=${limit}&offset=${offset}`
      );
      setRuns(data.runs);
      setTotal(data.total);
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load run history");
    } finally {
      setLoading(false);
    }
  }, [name, limit, offset]);

  useEffect(() => {
    load();
  }, [load]);

  if (loading) {
    return (
      <div className="py-4 flex justify-center">
        <Loader2 className="h-4 w-4 animate-spin text-muted-foreground" />
      </div>
    );
  }

  if (error) {
    return (
      <div className="py-2 px-4">
        <ErrorAlert message={error} onRetry={load} onDismiss={() => setError(null)} />
      </div>
    );
  }

  if (runs.length === 0) {
    return (
      <div className="py-4 text-center text-sm text-muted-foreground">
        No execution history yet
      </div>
    );
  }

  const hasMore = offset + limit < total;
  const hasPrev = offset > 0;

  return (
    <div>
      <table className="w-full text-xs">
        <thead>
          <tr className="border-b border-border text-left text-muted-foreground">
            <th className="px-4 py-2 font-medium">Started</th>
            <th className="px-4 py-2 font-medium">Duration</th>
            <th className="px-4 py-2 font-medium">Status</th>
            <th className="px-4 py-2 font-medium hidden md:table-cell">Error</th>
          </tr>
        </thead>
        <tbody>
          {runs.map((run) => (
            <tr key={run.id} className="border-b border-border/50 last:border-0">
              <td className="px-4 py-2 text-muted-foreground">
                <span title={formatTime(run.started_at)}>
                  {relativeTime(run.started_at)}
                </span>
              </td>
              <td className="px-4 py-2 font-mono">
                {formatDuration(run.duration_ms)}
              </td>
              <td className="px-4 py-2">
                {run.status === "success" ? (
                  <span className="inline-flex items-center gap-1 text-green-600">
                    <CheckCircle className="h-3 w-3" />
                    Success
                  </span>
                ) : (
                  <span className="inline-flex items-center gap-1 text-destructive">
                    <XCircle className="h-3 w-3" />
                    Error
                  </span>
                )}
              </td>
              <td className="px-4 py-2 max-w-[300px] truncate text-destructive hidden md:table-cell">
                {run.error || "—"}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
      {(hasPrev || hasMore) && (
        <div className="flex items-center justify-between px-4 py-2 border-t border-border/50">
          <span className="text-xs text-muted-foreground">
            {offset + 1}–{Math.min(offset + limit, total)} of {total}
          </span>
          <div className="flex gap-1">
            <Button
              variant="ghost"
              size="sm"
              onClick={() => setOffset(Math.max(0, offset - limit))}
              disabled={!hasPrev}
            >
              Previous
            </Button>
            <Button
              variant="ghost"
              size="sm"
              onClick={() => setOffset(offset + limit)}
              disabled={!hasMore}
            >
              Next
            </Button>
          </div>
        </div>
      )}
    </div>
  );
}

export default function CronPage() {
  const [entries, setEntries] = useState<CronEntry[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [acting, setActing] = useState<string | null>(null);
  const [expanded, setExpanded] = useState<string | null>(null);

  const load = useCallback(async () => {
    try {
      const data = await get<CronResponse>("/admin/cron");
      setEntries(data.entries);
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load cron jobs");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    load();
    const interval = setInterval(load, 10_000);
    return () => clearInterval(interval);
  }, [load]);

  async function action(name: string, act: string) {
    setActing(`${name}:${act}`);
    try {
      await post(`/admin/cron/${encodeURIComponent(name)}/${act}`);
      const labels: Record<string, string> = { trigger: "triggered", enable: "enabled", disable: "disabled" };
      toast.success(`Cron job ${labels[act] || act}`);
      await load();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : `Failed to ${act} job`);
    } finally {
      setActing(null);
    }
  }

  function toggleExpand(name: string) {
    setExpanded((prev) => (prev === name ? null : name));
  }

  return (
    <div className="p-6">
      <div className="mb-6 flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight">Cron Jobs</h1>
          <p className="text-sm text-muted-foreground">
            Scheduled tasks and their execution history
          </p>
        </div>
        <Button variant="outline" size="sm" onClick={load} disabled={loading}>
          <RotateCw className={`h-4 w-4 ${loading ? "animate-spin" : ""}`} />
          Refresh
        </Button>
      </div>

      {error && (
        <ErrorAlert message={error} onRetry={load} onDismiss={() => setError(null)} className="mb-6" />
      )}

      {loading && entries.length === 0 && <Spinner />}

      {!loading && entries.length === 0 && !error && (
        <EmptyState message="No cron jobs registered" />
      )}

      {entries.length > 0 && (
        <Card>
          <CardHeader>
            <CardTitle className="text-base">Registered Jobs</CardTitle>
          </CardHeader>
          <CardContent className="p-0">
            <div className="overflow-x-auto">
              <table className="w-full text-sm">
                <thead>
                  <tr className="border-b border-border text-left text-muted-foreground">
                    <th className="px-4 py-3 font-medium w-8"></th>
                    <th className="px-4 py-3 font-medium">Name</th>
                    <th className="px-4 py-3 font-medium hidden md:table-cell">Schedule</th>
                    <th className="px-4 py-3 font-medium">Status</th>
                    <th className="px-4 py-3 font-medium">Last Run</th>
                    <th className="px-4 py-3 font-medium hidden md:table-cell">Next Run</th>
                    <th className="px-4 py-3 font-medium hidden lg:table-cell">Last Error</th>
                    <th className="px-4 py-3 font-medium text-right">Actions</th>
                  </tr>
                </thead>
                <tbody>
                  {entries.map((entry) => (
                    <>
                      <tr
                        key={entry.name}
                        className={`border-b border-border cursor-pointer hover:bg-muted/50 ${
                          expanded === entry.name ? "bg-muted/30" : ""
                        }`}
                        onClick={() => toggleExpand(entry.name)}
                      >
                        <td className="px-4 py-3 text-muted-foreground">
                          {expanded === entry.name ? (
                            <ChevronDown className="h-4 w-4" />
                          ) : (
                            <ChevronRight className="h-4 w-4" />
                          )}
                        </td>
                        <td className="px-4 py-3 font-mono text-xs">{entry.name}</td>
                        <td className="px-4 py-3 font-mono text-xs hidden md:table-cell">{entry.schedule}</td>
                        <td className="px-4 py-3">
                          {entry.running ? (
                            <span className="inline-flex items-center gap-1 text-blue-600">
                              <Loader2 className="h-3 w-3 animate-spin" />
                              Running
                            </span>
                          ) : entry.enabled ? (
                            <span className="inline-flex items-center gap-1 text-green-600">
                              <CheckCircle className="h-3 w-3" />
                              Enabled
                            </span>
                          ) : (
                            <span className="inline-flex items-center gap-1 text-muted-foreground">
                              <XCircle className="h-3 w-3" />
                              Disabled
                            </span>
                          )}
                        </td>
                        <td className="px-4 py-3 text-muted-foreground">
                          <span title={formatTime(entry.last_run)}>
                            {entry.last_run ? relativeTime(entry.last_run) : "—"}
                          </span>
                        </td>
                        <td className="px-4 py-3 text-muted-foreground hidden md:table-cell">
                          <span title={formatTime(entry.next_run)}>
                            {entry.next_run ? relativeTime(entry.next_run) : "—"}
                          </span>
                        </td>
                        <td className="px-4 py-3 max-w-[200px] truncate text-destructive hidden lg:table-cell">
                          {entry.last_err || "—"}
                        </td>
                        <td className="px-4 py-3">
                          <div className="flex items-center justify-end gap-1" onClick={(e) => e.stopPropagation()}>
                            <Button
                              variant="ghost"
                              size="sm"
                              onClick={() => action(entry.name, "trigger")}
                              disabled={acting !== null}
                              title="Trigger now"
                            >
                              <Play className="h-3 w-3" />
                            </Button>
                            {entry.enabled ? (
                              <Button
                                variant="ghost"
                                size="sm"
                                onClick={() => action(entry.name, "disable")}
                                disabled={acting !== null}
                                title="Disable"
                              >
                                <Pause className="h-3 w-3" />
                              </Button>
                            ) : (
                              <Button
                                variant="ghost"
                                size="sm"
                                onClick={() => action(entry.name, "enable")}
                                disabled={acting !== null}
                                title="Enable"
                              >
                                <Clock className="h-3 w-3" />
                              </Button>
                            )}
                          </div>
                        </td>
                      </tr>
                      {expanded === entry.name && (
                        <tr key={`${entry.name}-history`} className="border-b border-border">
                          <td colSpan={8} className="bg-muted/20 p-0">
                            <div className="px-4 py-2 flex items-center gap-2 border-b border-border/50">
                              <History className="h-4 w-4 text-muted-foreground" />
                              <span className="text-sm font-medium">Execution History</span>
                            </div>
                            <RunHistory name={entry.name} />
                          </td>
                        </tr>
                      )}
                    </>
                  ))}
                </tbody>
              </table>
            </div>
          </CardContent>
        </Card>
      )}
    </div>
  );
}
