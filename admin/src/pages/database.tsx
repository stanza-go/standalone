import { useEffect, useState } from "react";
import { toast } from "sonner";
import { get, post } from "@/lib/api";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import {
  Database,
  Download,
  FileArchive,
  HardDrive,
  Layers,
  Loader2,
  RefreshCw,
} from "lucide-react";
import { Spinner } from "@/components/ui/spinner";
import { ErrorAlert } from "@/components/ui/error-alert";

interface DatabaseInfo {
  files: {
    db_size_bytes: number;
    wal_size_bytes: number;
    shm_size_bytes: number;
    path: string;
  };
  pragmas: {
    page_count: number;
    page_size: number;
    freelist_count: number;
    journal_mode: string;
  };
  tables: { name: string; row_count: number }[];
  migrations: { version: number; name: string; applied_at: string }[];
  backups: { name: string; size_bytes: number; created_at: string }[];
}

interface BackupResult {
  name: string;
  size_bytes: number;
  created_at: string;
}

function formatBytes(bytes: number): string {
  if (bytes === 0) return "0 B";
  const units = ["B", "KB", "MB", "GB"];
  const i = Math.floor(Math.log(bytes) / Math.log(1024));
  return `${(bytes / Math.pow(1024, i)).toFixed(1)} ${units[i]}`;
}

function formatDate(iso: string): string {
  return new Date(iso).toLocaleString();
}

export default function DatabasePage() {
  const [info, setInfo] = useState<DatabaseInfo | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [backingUp, setBackingUp] = useState(false);
  const [backupMsg, setBackupMsg] = useState<string | null>(null);
  const [downloading, setDownloading] = useState(false);

  async function load() {
    try {
      const data = await get<DatabaseInfo>("/admin/database");
      setInfo(data);
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load");
    }
  }

  useEffect(() => {
    load();
  }, []);

  async function triggerBackup() {
    setBackingUp(true);
    setBackupMsg(null);
    try {
      const result = await post<BackupResult>("/admin/database/backup");
      setBackupMsg(`Backup created: ${result.name} (${formatBytes(result.size_bytes)})`);
      toast.success("Backup created successfully");
      load();
    } catch (err) {
      const msg = err instanceof Error ? err.message : "Backup failed";
      setBackupMsg(msg);
      toast.error(msg);
    } finally {
      setBackingUp(false);
    }
  }

  async function downloadDB() {
    setDownloading(true);
    try {
      const res = await fetch("/api/admin/database/download", {
        credentials: "include",
      });
      if (!res.ok) {
        const data = await res.json().catch(() => ({ error: "Download failed" }));
        throw new Error(data.error ?? "Download failed");
      }
      const blob = await res.blob();
      const url = URL.createObjectURL(blob);
      const a = document.createElement("a");
      a.href = url;
      a.download = "database.sqlite";
      document.body.appendChild(a);
      a.click();
      a.remove();
      URL.revokeObjectURL(url);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Download failed");
    } finally {
      setDownloading(false);
    }
  }

  return (
    <div className="p-6">
      <div className="mb-6 flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight">Database</h1>
          <p className="text-sm text-muted-foreground">
            SQLite statistics, migrations, and backups
          </p>
        </div>
        <div className="flex gap-2">
          <Button variant="outline" size="sm" onClick={load}>
            <RefreshCw className="mr-2 h-3.5 w-3.5" />
            Refresh
          </Button>
          <Button variant="outline" size="sm" onClick={downloadDB} disabled={downloading}>
            {downloading ? (
              <Loader2 className="mr-2 h-3.5 w-3.5 animate-spin" />
            ) : (
              <Download className="mr-2 h-3.5 w-3.5" />
            )}
            Download
          </Button>
          <Button size="sm" onClick={triggerBackup} disabled={backingUp}>
            {backingUp ? (
              <Loader2 className="mr-2 h-3.5 w-3.5 animate-spin" />
            ) : (
              <FileArchive className="mr-2 h-3.5 w-3.5" />
            )}
            Backup Now
          </Button>
        </div>
      </div>

      {error && (
        <ErrorAlert message={error} onRetry={load} onDismiss={() => setError(null)} className="mb-6" />
      )}

      {backupMsg && (
        <div className="mb-6 rounded-md border border-border bg-muted/50 p-3 text-sm">
          {backupMsg}
        </div>
      )}

      {!info && !error && <Spinner />}

      {info && (
        <div className="space-y-6">
          {/* Stats Cards */}
          <section>
            <h2 className="mb-3 text-sm font-medium text-muted-foreground uppercase tracking-wider">
              Storage
            </h2>
            <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
              <StatCard
                title="Database Size"
                value={formatBytes(info.files.db_size_bytes)}
                icon={<HardDrive className="h-4 w-4" />}
              />
              <StatCard
                title="WAL Size"
                value={formatBytes(info.files.wal_size_bytes)}
                icon={<Database className="h-4 w-4" />}
              />
              <StatCard
                title="Journal Mode"
                value={info.pragmas.journal_mode.toUpperCase()}
                icon={<Layers className="h-4 w-4" />}
              />
              <StatCard
                title="Page Count"
                value={String(info.pragmas.page_count)}
                description={`${formatBytes(info.pragmas.page_size)} per page · ${info.pragmas.freelist_count} free`}
                icon={<Database className="h-4 w-4" />}
              />
            </div>
          </section>

          {/* Tables */}
          <section>
            <h2 className="mb-3 text-sm font-medium text-muted-foreground uppercase tracking-wider">
              Tables ({info.tables.length})
            </h2>
            <div className="rounded-md border border-border">
              <table className="w-full text-sm">
                <thead>
                  <tr className="border-b border-border bg-muted/50">
                    <th className="px-4 py-2 text-left font-medium">Name</th>
                    <th className="px-4 py-2 text-right font-medium">Rows</th>
                  </tr>
                </thead>
                <tbody>
                  {info.tables.map((t) => (
                    <tr
                      key={t.name}
                      className="border-b border-border last:border-0"
                    >
                      <td className="px-4 py-2 font-mono text-xs">{t.name}</td>
                      <td className="px-4 py-2 text-right tabular-nums">
                        {t.row_count.toLocaleString()}
                      </td>
                    </tr>
                  ))}
                  {info.tables.length === 0 && (
                    <tr>
                      <td
                        colSpan={2}
                        className="px-4 py-4 text-center text-muted-foreground"
                      >
                        No tables
                      </td>
                    </tr>
                  )}
                </tbody>
              </table>
            </div>
          </section>

          {/* Migrations */}
          <section>
            <h2 className="mb-3 text-sm font-medium text-muted-foreground uppercase tracking-wider">
              Migrations ({info.migrations.length})
            </h2>
            <div className="rounded-md border border-border">
              <table className="w-full text-sm">
                <thead>
                  <tr className="border-b border-border bg-muted/50">
                    <th className="px-4 py-2 text-left font-medium">Version</th>
                    <th className="px-4 py-2 text-left font-medium">Name</th>
                    <th className="px-4 py-2 text-right font-medium">Applied</th>
                  </tr>
                </thead>
                <tbody>
                  {info.migrations.map((m) => (
                    <tr
                      key={m.version}
                      className="border-b border-border last:border-0"
                    >
                      <td className="px-4 py-2 font-mono text-xs">
                        {m.version}
                      </td>
                      <td className="px-4 py-2">{m.name}</td>
                      <td className="px-4 py-2 text-right text-muted-foreground text-xs">
                        {formatDate(m.applied_at)}
                      </td>
                    </tr>
                  ))}
                  {info.migrations.length === 0 && (
                    <tr>
                      <td
                        colSpan={3}
                        className="px-4 py-4 text-center text-muted-foreground"
                      >
                        No migrations applied
                      </td>
                    </tr>
                  )}
                </tbody>
              </table>
            </div>
          </section>

          {/* Backups */}
          <section>
            <h2 className="mb-3 text-sm font-medium text-muted-foreground uppercase tracking-wider">
              Backups ({info.backups.length})
            </h2>
            <div className="rounded-md border border-border">
              <table className="w-full text-sm">
                <thead>
                  <tr className="border-b border-border bg-muted/50">
                    <th className="px-4 py-2 text-left font-medium">
                      <FileArchive className="mr-1.5 inline h-3.5 w-3.5" />
                      File
                    </th>
                    <th className="px-4 py-2 text-right font-medium">Size</th>
                    <th className="px-4 py-2 text-right font-medium">Created</th>
                  </tr>
                </thead>
                <tbody>
                  {info.backups.map((b) => (
                    <tr
                      key={b.name}
                      className="border-b border-border last:border-0"
                    >
                      <td className="px-4 py-2 font-mono text-xs">{b.name}</td>
                      <td className="px-4 py-2 text-right tabular-nums">
                        {formatBytes(b.size_bytes)}
                      </td>
                      <td className="px-4 py-2 text-right text-muted-foreground text-xs">
                        {formatDate(b.created_at)}
                      </td>
                    </tr>
                  ))}
                  {info.backups.length === 0 && (
                    <tr>
                      <td
                        colSpan={3}
                        className="px-4 py-4 text-center text-muted-foreground"
                      >
                        No backups yet
                      </td>
                    </tr>
                  )}
                </tbody>
              </table>
            </div>
          </section>

          {/* DB Path */}
          <div className="text-xs text-muted-foreground">
            Path: <span className="font-mono">{info.files.path}</span>
          </div>
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
