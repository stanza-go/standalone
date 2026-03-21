import { useEffect, useState, useCallback } from "react";
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

export default function CronPage() {
  const [entries, setEntries] = useState<CronEntry[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [acting, setActing] = useState<string | null>(null);

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
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : `Failed to ${act} job`);
    } finally {
      setActing(null);
    }
  }

  return (
    <div className="p-6">
      <div className="mb-6 flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight">Cron Jobs</h1>
          <p className="text-sm text-muted-foreground">
            Scheduled tasks and their status
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
                    <th className="px-4 py-3 font-medium">Name</th>
                    <th className="px-4 py-3 font-medium">Schedule</th>
                    <th className="px-4 py-3 font-medium">Status</th>
                    <th className="px-4 py-3 font-medium">Last Run</th>
                    <th className="px-4 py-3 font-medium">Next Run</th>
                    <th className="px-4 py-3 font-medium">Last Error</th>
                    <th className="px-4 py-3 font-medium text-right">Actions</th>
                  </tr>
                </thead>
                <tbody>
                  {entries.map((entry) => (
                    <tr key={entry.name} className="border-b border-border last:border-0">
                      <td className="px-4 py-3 font-mono text-xs">{entry.name}</td>
                      <td className="px-4 py-3 font-mono text-xs">{entry.schedule}</td>
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
                      <td className="px-4 py-3 text-muted-foreground">
                        <span title={formatTime(entry.next_run)}>
                          {entry.next_run ? relativeTime(entry.next_run) : "—"}
                        </span>
                      </td>
                      <td className="px-4 py-3 max-w-[200px] truncate text-destructive">
                        {entry.last_err || "—"}
                      </td>
                      <td className="px-4 py-3">
                        <div className="flex items-center justify-end gap-1">
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
