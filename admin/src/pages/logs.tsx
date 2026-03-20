import { useEffect, useState, useCallback, useRef } from "react";
import { get } from "@/lib/api";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  RotateCw,
  ChevronDown,
  ChevronRight,
  Pause,
  Play,
} from "lucide-react";

interface LogEntry {
  time: string;
  level: string;
  msg: string;
  [key: string]: unknown;
}

interface LogsResponse {
  entries: LogEntry[];
  file: string;
  total: number;
}

interface LogFile {
  name: string;
  size: number;
}

interface FilesResponse {
  files: LogFile[];
}

const LEVELS = ["", "debug", "info", "warn", "error"] as const;

function formatTime(iso: string): string {
  if (!iso) return "—";
  const d = new Date(iso);
  return d.toLocaleTimeString([], { hour12: false }) + "." + String(d.getMilliseconds()).padStart(3, "0");
}

function formatDate(iso: string): string {
  if (!iso) return "";
  return new Date(iso).toLocaleDateString();
}

function formatSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}

function LevelBadge({ level }: { level: string }) {
  const styles: Record<string, string> = {
    debug: "bg-gray-100 text-gray-600",
    info: "bg-blue-100 text-blue-700",
    warn: "bg-yellow-100 text-yellow-700",
    error: "bg-red-100 text-red-700",
  };
  return (
    <span
      className={`inline-block w-14 rounded-full px-2 py-0.5 text-center text-xs font-medium ${styles[level] || "bg-gray-100 text-gray-600"}`}
    >
      {level}
    </span>
  );
}

function ExtraFields({ entry }: { entry: LogEntry }) {
  const reserved = new Set(["time", "level", "msg"]);
  const extra = Object.entries(entry).filter(([k]) => !reserved.has(k));
  if (extra.length === 0) return <span className="text-muted-foreground">No extra fields</span>;
  return (
    <div className="space-y-1">
      {extra.map(([k, v]) => (
        <div key={k} className="flex gap-2">
          <span className="font-medium text-muted-foreground">{k}:</span>
          <span className="break-all">
            {typeof v === "string" ? v : JSON.stringify(v)}
          </span>
        </div>
      ))}
    </div>
  );
}

export default function LogsPage() {
  const [entries, setEntries] = useState<LogEntry[]>([]);
  const [files, setFiles] = useState<LogFile[]>([]);
  const [selectedFile, setSelectedFile] = useState("stanza.log");
  const [level, setLevel] = useState("");
  const [search, setSearch] = useState("");
  const [searchInput, setSearchInput] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [expanded, setExpanded] = useState<Set<number>>(new Set());
  const [autoRefresh, setAutoRefresh] = useState(true);
  const [total, setTotal] = useState(0);
  const intervalRef = useRef<ReturnType<typeof setInterval> | null>(null);

  const loadFiles = useCallback(async () => {
    try {
      const data = await get<FilesResponse>("/admin/logs/files");
      setFiles(data.files);
    } catch {
      // Files list is optional — don't block main view.
    }
  }, []);

  const loadEntries = useCallback(async () => {
    try {
      const params = new URLSearchParams({ limit: "200", file: selectedFile });
      if (level) params.set("level", level);
      if (search) params.set("search", search);
      const data = await get<LogsResponse>(`/admin/logs?${params}`);
      setEntries(data.entries);
      setTotal(data.total);
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load logs");
    }
  }, [selectedFile, level, search]);

  const loadAll = useCallback(async () => {
    setLoading(true);
    await Promise.all([loadEntries(), loadFiles()]);
    setLoading(false);
  }, [loadEntries, loadFiles]);

  useEffect(() => {
    loadAll();
  }, [loadAll]);

  useEffect(() => {
    if (intervalRef.current) {
      clearInterval(intervalRef.current);
      intervalRef.current = null;
    }
    if (autoRefresh) {
      intervalRef.current = setInterval(loadEntries, 5_000);
    }
    return () => {
      if (intervalRef.current) clearInterval(intervalRef.current);
    };
  }, [autoRefresh, loadEntries]);

  function toggleExpanded(idx: number) {
    setExpanded((prev) => {
      const next = new Set(prev);
      if (next.has(idx)) next.delete(idx);
      else next.add(idx);
      return next;
    });
  }

  function handleSearch(e: React.FormEvent) {
    e.preventDefault();
    setSearch(searchInput);
  }

  // Group entries by date for visual separation.
  let lastDate = "";

  return (
    <div className="p-6">
      <div className="mb-6 flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight">Logs</h1>
          <p className="text-sm text-muted-foreground">
            {total} entries in {selectedFile}
          </p>
        </div>
        <div className="flex items-center gap-2">
          <Button
            variant={autoRefresh ? "default" : "outline"}
            size="sm"
            onClick={() => setAutoRefresh(!autoRefresh)}
            title={autoRefresh ? "Pause auto-refresh" : "Resume auto-refresh"}
          >
            {autoRefresh ? <Pause className="h-4 w-4" /> : <Play className="h-4 w-4" />}
            {autoRefresh ? "Live" : "Paused"}
          </Button>
          <Button variant="outline" size="sm" onClick={loadAll} disabled={loading}>
            <RotateCw className={`h-4 w-4 ${loading ? "animate-spin" : ""}`} />
            Refresh
          </Button>
        </div>
      </div>

      {error && (
        <div className="mb-6 rounded-md border border-destructive/50 bg-destructive/5 p-3 text-sm text-destructive">
          {error}
        </div>
      )}

      <Card className="mb-4">
        <CardContent className="flex flex-wrap items-center gap-3 py-3">
          {/* Level filter */}
          <div className="flex items-center gap-1">
            {LEVELS.map((l) => (
              <Button
                key={l || "all"}
                variant={level === l ? "default" : "ghost"}
                size="sm"
                onClick={() => setLevel(l)}
              >
                {l || "All"}
              </Button>
            ))}
          </div>

          <div className="h-6 w-px bg-border" />

          {/* Search */}
          <form onSubmit={handleSearch} className="flex items-center gap-2">
            <Input
              placeholder="Search messages..."
              value={searchInput}
              onChange={(e) => setSearchInput(e.target.value)}
              className="h-8 w-48"
            />
            <Button type="submit" variant="outline" size="sm">
              Search
            </Button>
            {search && (
              <Button
                variant="ghost"
                size="sm"
                onClick={() => {
                  setSearch("");
                  setSearchInput("");
                }}
              >
                Clear
              </Button>
            )}
          </form>

          <div className="h-6 w-px bg-border" />

          {/* File selector */}
          <select
            value={selectedFile}
            onChange={(e) => setSelectedFile(e.target.value)}
            className="h-8 rounded-md border border-input bg-background px-2 text-sm"
          >
            {files.map((f) => (
              <option key={f.name} value={f.name}>
                {f.name} ({formatSize(f.size)})
              </option>
            ))}
            {files.length === 0 && <option value="stanza.log">stanza.log</option>}
          </select>
        </CardContent>
      </Card>

      <Card>
        <CardHeader className="pb-0">
          <CardTitle className="text-base">
            Log Entries
            {entries.length > 0 && (
              <span className="ml-2 text-sm font-normal text-muted-foreground">
                showing {entries.length} of {total}
              </span>
            )}
          </CardTitle>
        </CardHeader>
        <CardContent className="p-0">
          {loading && entries.length === 0 ? (
            <div className="p-4 text-sm text-muted-foreground">Loading...</div>
          ) : entries.length === 0 ? (
            <div className="p-4 text-sm text-muted-foreground">No log entries found.</div>
          ) : (
            <div className="overflow-x-auto">
              <table className="w-full text-sm">
                <thead>
                  <tr className="border-b border-border text-left text-muted-foreground">
                    <th className="w-8 px-2 py-3"></th>
                    <th className="px-4 py-3 font-medium">Time</th>
                    <th className="px-4 py-3 font-medium">Level</th>
                    <th className="px-4 py-3 font-medium">Message</th>
                  </tr>
                </thead>
                <tbody>
                  {entries.map((entry, idx) => {
                    const date = formatDate(entry.time);
                    const showDateSep = date !== lastDate;
                    lastDate = date;
                    const isExpanded = expanded.has(idx);

                    return (
                      <>
                        {showDateSep && (
                          <tr key={`date-${date}`}>
                            <td
                              colSpan={4}
                              className="border-b border-border bg-muted/40 px-4 py-1.5 text-xs font-medium text-muted-foreground"
                            >
                              {date}
                            </td>
                          </tr>
                        )}
                        <tr
                          key={idx}
                          className="cursor-pointer border-b border-border last:border-0 hover:bg-muted/30"
                          onClick={() => toggleExpanded(idx)}
                        >
                          <td className="px-2 py-2.5 text-muted-foreground">
                            {isExpanded ? (
                              <ChevronDown className="h-3.5 w-3.5" />
                            ) : (
                              <ChevronRight className="h-3.5 w-3.5" />
                            )}
                          </td>
                          <td className="whitespace-nowrap px-4 py-2.5 font-mono text-xs text-muted-foreground">
                            {formatTime(entry.time)}
                          </td>
                          <td className="px-4 py-2.5">
                            <LevelBadge level={entry.level} />
                          </td>
                          <td className="max-w-[600px] truncate px-4 py-2.5">{entry.msg}</td>
                        </tr>
                        {isExpanded && (
                          <tr key={`detail-${idx}`} className="border-b border-border bg-muted/20">
                            <td></td>
                            <td colSpan={3} className="px-4 py-3 text-xs">
                              <ExtraFields entry={entry} />
                            </td>
                          </tr>
                        )}
                      </>
                    );
                  })}
                </tbody>
              </table>
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
