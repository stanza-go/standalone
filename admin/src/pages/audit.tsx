import { useCallback, useEffect, useState } from "react";
import { get } from "@/lib/api";
import { useDebounce } from "@/lib/use-debounce";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  Search,
  ChevronDown,
  ChevronUp,
  Filter,
  Calendar,
  X,
} from "lucide-react";
import { TableSkeleton } from "@/components/ui/skeleton";
import { ErrorAlert } from "@/components/ui/error-alert";
import { Pagination } from "@/components/ui/pagination";
import { TableEmptyRow } from "@/components/ui/empty-state";
import { SortableHeader, useSort } from "@/components/ui/sortable-header";

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
  "cron.trigger": "Triggered cron job",
  "cron.enable": "Enabled cron job",
  "cron.disable": "Disabled cron job",
  "job.retry": "Retried job",
  "job.cancel": "Cancelled job",
};

const ACTION_COLORS: Record<string, string> = {
  create: "bg-green-100 text-green-700 dark:bg-green-500/10 dark:text-green-400",
  update: "bg-blue-100 text-blue-700 dark:bg-blue-500/10 dark:text-blue-400",
  delete: "bg-red-100 text-red-700 dark:bg-red-500/10 dark:text-red-400",
  revoke: "bg-orange-100 text-orange-700 dark:bg-orange-500/10 dark:text-orange-400",
  impersonate: "bg-amber-100 text-amber-700 dark:bg-amber-500/10 dark:text-amber-400",
  trigger: "bg-purple-100 text-purple-700 dark:bg-purple-500/10 dark:text-purple-400",
  enable: "bg-green-100 text-green-700 dark:bg-green-500/10 dark:text-green-400",
  disable: "bg-red-100 text-red-700 dark:bg-red-500/10 dark:text-red-400",
  retry: "bg-blue-100 text-blue-700 dark:bg-blue-500/10 dark:text-blue-400",
  cancel: "bg-orange-100 text-orange-700 dark:bg-orange-500/10 dark:text-orange-400",
};

function actionColor(action: string): string {
  const verb = action.split(".")[1] ?? "";
  return ACTION_COLORS[verb] ?? "bg-gray-100 text-gray-700 dark:bg-gray-500/10 dark:text-gray-400";
}

function formatTime(iso: string): string {
  if (!iso) return "\u2014";
  const d = new Date(iso);
  return d.toLocaleDateString() + " " + d.toLocaleTimeString();
}

function formatRelativeTime(iso: string): string {
  const diff = (Date.now() - new Date(iso).getTime()) / 1000;
  if (diff < 60) return "just now";
  if (diff < 3600) return `${Math.round(diff / 60)}m ago`;
  if (diff < 86400) return `${Math.round(diff / 3600)}h ago`;
  return `${Math.round(diff / 86400)}d ago`;
}

export default function AuditPage() {
  const [entries, setEntries] = useState<AuditEntry[]>([]);
  const [total, setTotal] = useState(0);
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(true);

  // Pagination.
  const [page, setPage] = useState(0);
  const pageSize = 30;

  // Filters.
  const [searchInput, setSearchInput] = useState("");
  const search = useDebounce(searchInput, 300);
  const [actionFilter, setActionFilter] = useState("");
  const [dateFrom, setDateFrom] = useState("");
  const [dateTo, setDateTo] = useState("");

  // Sort.
  const [sort, toggleSort] = useSort("id", "desc");

  // Expanded rows.
  const [expanded, setExpanded] = useState<Set<number>>(new Set());

  const load = useCallback(async () => {
    try {
      const params = new URLSearchParams();
      params.set("limit", String(pageSize));
      params.set("offset", String(page * pageSize));
      if (search) params.set("search", search);
      if (actionFilter) params.set("action", actionFilter);
      if (dateFrom) params.set("from", dateFrom + "T00:00:00Z");
      if (dateTo) params.set("to", dateTo + "T23:59:59Z");
      params.set("sort", sort.column);
      params.set("order", sort.direction);

      const data = await get<{ entries: AuditEntry[]; total: number }>(
        `/admin/audit?${params}`
      );
      setEntries(data.entries);
      setTotal(data.total);
      setError("");
    } catch (e: any) {
      setError(e.message || "Failed to load audit log");
    } finally {
      setLoading(false);
    }
  }, [page, search, actionFilter, dateFrom, dateTo, sort.column, sort.direction]);

  useEffect(() => {
    load();
  }, [load]);

  // Reset to first page when search changes.
  useEffect(() => {
    setPage(0);
  }, [search]);

  function toggleExpand(id: number) {
    setExpanded((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  }

  // Collect unique actions for filter dropdown.
  const uniqueActions = Object.keys(ACTION_LABELS);

  const totalPages = Math.ceil(total / pageSize);

  if (loading) {
    return (
      <div className="p-6">
        <h1 className="text-2xl font-bold mb-6">Audit Log</h1>
        <TableSkeleton columns={[
          { width: "w-6" },
          { width: "w-24" },
          { width: "w-20" },
          { width: "w-24" },
          { width: "w-20", hidden: "hidden md:table-cell" },
          { width: "w-24", hidden: "hidden lg:table-cell" },
          { width: "w-20", hidden: "hidden lg:table-cell" },
        ]} rows={8} />
      </div>
    );
  }

  return (
    <div className="p-6">
      <div className="mb-6">
        <h1 className="text-2xl font-semibold tracking-tight">Audit Log</h1>
        <p className="text-sm text-muted-foreground">
          {total} event{total !== 1 ? "s" : ""} recorded
        </p>
      </div>

      {error && (
        <ErrorAlert
          message={error}
          onRetry={load}
          onDismiss={() => setError("")}
          className="mb-4"
        />
      )}

      {/* Filters */}
      <div className="mb-4 flex flex-wrap gap-2 items-center">
        <div className="relative flex-1 min-w-[200px] max-w-sm">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
          <Input
            placeholder="Search actions or details..."
            value={searchInput}
            onChange={(e) => setSearchInput(e.target.value)}
            className="pl-9 pr-9"
          />
          {searchInput && (
            <button
              onClick={() => setSearchInput("")}
              className="absolute right-3 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground"
            >
              <X className="h-4 w-4" />
            </button>
          )}
        </div>

        <div className="flex items-center gap-2">
          <Filter className="h-4 w-4 text-muted-foreground" />
          <select
            value={actionFilter}
            onChange={(e) => {
              setActionFilter(e.target.value);
              setPage(0);
            }}
            className="h-9 rounded-md border border-input bg-background px-3 text-sm"
          >
            <option value="">All actions</option>
            {uniqueActions.map((a) => (
              <option key={a} value={a}>
                {ACTION_LABELS[a] || a}
              </option>
            ))}
          </select>
        </div>

        <div className="flex items-center gap-2">
          <Calendar className="h-4 w-4 text-muted-foreground" />
          <Input
            type="date"
            value={dateFrom}
            onChange={(e) => {
              setDateFrom(e.target.value);
              setPage(0);
            }}
            className="h-9 w-[140px] text-sm"
            title="From date"
          />
          <span className="text-xs text-muted-foreground">&ndash;</span>
          <Input
            type="date"
            value={dateTo}
            onChange={(e) => {
              setDateTo(e.target.value);
              setPage(0);
            }}
            className="h-9 w-[140px] text-sm"
            title="To date"
          />
        </div>

        {(search || actionFilter || dateFrom || dateTo) && (
          <Button
            variant="ghost"
            size="sm"
            onClick={() => {
              setSearchInput("");
              setActionFilter("");
              setDateFrom("");
              setDateTo("");
              setPage(0);
            }}
          >
            <X className="h-4 w-4 mr-1" />
            Clear filters
          </Button>
        )}
      </div>

      <div className="border rounded-lg overflow-hidden">
        <table className="w-full text-sm">
          <thead>
            <tr className="bg-muted/50 border-b">
              <SortableHeader label="#" column="id" sort={sort} onSort={toggleSort} className="w-8" />
              <SortableHeader label="Time" column="created_at" sort={sort} onSort={toggleSort} />
              <th className="text-left p-3 font-medium">Admin</th>
              <SortableHeader label="Action" column="action" sort={sort} onSort={toggleSort} />
              <SortableHeader label="Target" column="entity_type" sort={sort} onSort={toggleSort} className="hidden md:table-cell" />
              <th className="text-left p-3 font-medium hidden lg:table-cell">Details</th>
              <th className="text-left p-3 font-medium hidden lg:table-cell">IP</th>
            </tr>
          </thead>
          <tbody>
            {entries.length === 0 ? (
              <TableEmptyRow
                colSpan={7}
                message={
                  search || actionFilter || dateFrom || dateTo
                    ? "No events match your filters"
                    : "No audit events recorded yet"
                }
              />
            ) : (
              entries.map((entry) => {
                const isExpanded = expanded.has(entry.id);
                return (
                  <tr
                    key={entry.id}
                    className="border-b last:border-0 hover:bg-muted/30 cursor-pointer"
                    onClick={() => toggleExpand(entry.id)}
                  >
                    <td className="p-3">
                      {isExpanded ? (
                        <ChevronUp className="h-4 w-4 text-muted-foreground" />
                      ) : (
                        <ChevronDown className="h-4 w-4 text-muted-foreground" />
                      )}
                    </td>
                    <td className="p-3">
                      <div className="text-xs text-muted-foreground" title={formatTime(entry.created_at)}>
                        {formatRelativeTime(entry.created_at)}
                      </div>
                    </td>
                    <td className="p-3">
                      <div className="font-medium text-xs">
                        {entry.admin_name || entry.admin_email || `Admin #${entry.admin_id}`}
                      </div>
                      {entry.admin_name && entry.admin_email && (
                        <div className="text-xs text-muted-foreground">{entry.admin_email}</div>
                      )}
                    </td>
                    <td className="p-3">
                      <span
                        className={`inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium ${actionColor(entry.action)}`}
                      >
                        {ACTION_LABELS[entry.action] || entry.action}
                      </span>
                    </td>
                    <td className="p-3 hidden md:table-cell">
                      {entry.entity_type && (
                        <span className="text-xs font-mono text-muted-foreground">
                          {entry.entity_type}
                          {entry.entity_id ? `#${entry.entity_id}` : ""}
                        </span>
                      )}
                    </td>
                    <td className="p-3 hidden lg:table-cell">
                      <span className="text-xs text-muted-foreground truncate max-w-[200px] block">
                        {entry.details || "\u2014"}
                      </span>
                    </td>
                    <td className="p-3 hidden lg:table-cell">
                      <span className="text-xs font-mono text-muted-foreground">
                        {entry.ip_address}
                      </span>
                    </td>
                  </tr>
                );
              })
            )}
          </tbody>
        </table>

        {/* Expanded detail panels rendered outside the table for proper layout */}
        {entries
          .filter((e) => expanded.has(e.id))
          .map((entry) => (
            <div
              key={`detail-${entry.id}`}
              className="border-t bg-muted/20 px-6 py-3 text-xs space-y-1"
            >
              <div className="grid grid-cols-2 gap-x-6 gap-y-1 sm:grid-cols-4">
                <div>
                  <span className="text-muted-foreground">Event ID:</span>{" "}
                  <span className="font-mono">{entry.id}</span>
                </div>
                <div>
                  <span className="text-muted-foreground">Admin ID:</span>{" "}
                  <span className="font-mono">{entry.admin_id}</span>
                </div>
                <div>
                  <span className="text-muted-foreground">Action:</span>{" "}
                  <span className="font-mono">{entry.action}</span>
                </div>
                <div>
                  <span className="text-muted-foreground">Time:</span>{" "}
                  {formatTime(entry.created_at)}
                </div>
                <div>
                  <span className="text-muted-foreground">Target:</span>{" "}
                  <span className="font-mono">
                    {entry.entity_type}
                    {entry.entity_id ? `#${entry.entity_id}` : ""}
                  </span>
                </div>
                <div>
                  <span className="text-muted-foreground">IP Address:</span>{" "}
                  <span className="font-mono">{entry.ip_address}</span>
                </div>
                {entry.details && (
                  <div className="col-span-2">
                    <span className="text-muted-foreground">Details:</span>{" "}
                    {entry.details}
                  </div>
                )}
              </div>
            </div>
          ))}
      </div>

      {/* Pagination */}
      <Pagination
        page={page}
        totalPages={totalPages}
        total={total}
        pageSize={pageSize}
        onPrev={() => setPage(page - 1)}
        onNext={() => setPage(page + 1)}
      />
    </div>
  );
}
