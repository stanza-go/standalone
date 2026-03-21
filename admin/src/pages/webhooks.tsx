import { useCallback, useEffect, useState } from "react";
import { useNavigate } from "react-router";
import { toast } from "sonner";
import { get, post, put, del, downloadCSV, ApiError } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Dialog,
  DialogHeader,
  DialogTitle,
  DialogCloseButton,
  DialogBody,
  DialogFooter,
} from "@/components/ui/dialog";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { Plus, Pencil, Trash2, Copy, Check, Search, X, ExternalLink, Download } from "lucide-react";
import { TableSkeleton } from "@/components/ui/skeleton";
import { ErrorAlert } from "@/components/ui/error-alert";
import { Pagination } from "@/components/ui/pagination";
import { TableEmptyRow } from "@/components/ui/empty-state";
import { SortableHeader, useSort } from "@/components/ui/sortable-header";

interface Webhook {
  id: number;
  url: string;
  secret: string;
  description: string;
  events: string[];
  is_active: boolean;
  created_by: number;
  created_at: string;
  updated_at: string;
}

export default function WebhooksPage() {
  const navigate = useNavigate();
  const [webhooks, setWebhooks] = useState<Webhook[]>([]);
  const [total, setTotal] = useState(0);
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(true);
  const [acting, setActing] = useState<number | null>(null);

  // Pagination.
  const [page, setPage] = useState(0);
  const pageSize = 20;

  // Search.
  const [searchInput, setSearchInput] = useState("");

  // Sort.
  const [sort, toggleSort] = useSort("created_at", "desc");

  // Dialog state.
  const [dialogOpen, setDialogOpen] = useState(false);
  const [editing, setEditing] = useState<Webhook | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<Webhook | null>(null);

  // Form state.
  const [url, setUrl] = useState("");
  const [description, setDescription] = useState("");
  const [events, setEvents] = useState("");
  const [isActive, setIsActive] = useState(true);
  const [formError, setFormError] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [exporting, setExporting] = useState(false);

  // Created webhook secret reveal.
  const [createdWebhook, setCreatedWebhook] = useState<Webhook | null>(null);
  const [copied, setCopied] = useState(false);

  const load = useCallback(async () => {
    try {
      const params = new URLSearchParams();
      params.set("limit", String(pageSize));
      params.set("offset", String(page * pageSize));
      if (searchInput) params.set("search", searchInput);
      params.set("sort", sort.column);
      params.set("order", sort.direction);

      const data = await get<{ webhooks: Webhook[]; total: number }>(
        `/admin/webhooks?${params}`
      );
      setWebhooks(data.webhooks);
      setTotal(data.total);
      setError("");
    } catch (e: any) {
      setError(e.message || "Failed to load webhooks");
    } finally {
      setLoading(false);
    }
  }, [page, searchInput, sort.column, sort.direction]);

  useEffect(() => {
    load();
    const interval = setInterval(load, 30000);
    return () => clearInterval(interval);
  }, [load]);

  useEffect(() => {
    setPage(0);
  }, [searchInput]);

  function openCreate() {
    setEditing(null);
    setUrl("");
    setDescription("");
    setEvents("*");
    setIsActive(true);
    setFormError("");
    setDialogOpen(true);
  }

  function openEdit(wh: Webhook) {
    setEditing(wh);
    setUrl(wh.url);
    setDescription(wh.description);
    setEvents(wh.events.join(", "));
    setIsActive(wh.is_active);
    setFormError("");
    setDialogOpen(true);
  }

  function closeDialog() {
    setDialogOpen(false);
    setEditing(null);
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setFormError("");
    setSubmitting(true);

    const eventsList = events
      .split(",")
      .map((s) => s.trim())
      .filter(Boolean);

    try {
      if (editing) {
        await put(`/admin/webhooks/${editing.id}`, {
          url,
          description,
          events: eventsList,
          is_active: isActive,
        });
        toast.success("Webhook updated");
      } else {
        const data = await post<Webhook>("/admin/webhooks", {
          url,
          description,
          events: eventsList,
        });
        setCreatedWebhook(data);
        setCopied(false);
        toast.success("Webhook created");
      }
      closeDialog();
      await load();
    } catch (err) {
      if (err instanceof ApiError) {
        setFormError(err.message);
      } else {
        setFormError("Operation failed");
      }
    } finally {
      setSubmitting(false);
    }
  }

  async function handleDelete() {
    if (!deleteTarget) return;
    const id = deleteTarget.id;
    setActing(id);
    try {
      await del(`/admin/webhooks/${id}`);
      setDeleteTarget(null);
      toast.success("Webhook deleted");
      await load();
    } catch (e: any) {
      toast.error(e.message || "Failed to delete webhook");
    } finally {
      setActing(null);
    }
  }

  async function copySecret(secret: string) {
    await navigator.clipboard.writeText(secret);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  }

  async function handleExport() {
    setExporting(true);
    try {
      const params = new URLSearchParams();
      if (searchInput) params.set("search", searchInput);
      params.set("sort", sort.column);
      params.set("order", sort.direction);
      await downloadCSV(`/admin/webhooks/export?${params}`);
    } catch {
      toast.error("Failed to export webhooks");
    } finally {
      setExporting(false);
    }
  }

  function formatTime(iso: string): string {
    if (!iso) return "\u2014";
    const d = new Date(iso);
    return d.toLocaleDateString() + " " + d.toLocaleTimeString();
  }

  const totalPages = Math.ceil(total / pageSize);

  if (loading) {
    return (
      <div className="p-6">
        <h1 className="text-2xl font-bold mb-6">Webhooks</h1>
        <TableSkeleton columns={[
          { width: "w-40" },
          { width: "w-32", hidden: "hidden md:table-cell" },
          { width: "w-24", hidden: "hidden lg:table-cell" },
          { width: "w-16" },
          { width: "w-24", hidden: "hidden md:table-cell" },
          { width: "w-20" },
        ]} />
      </div>
    );
  }

  return (
    <div className="p-6">
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-2xl font-bold">Webhooks</h1>
          <p className="text-sm text-muted-foreground mt-1">
            {total} webhook{total !== 1 ? "s" : ""}
          </p>
        </div>
        <div className="flex gap-2">
          <Button variant="outline" onClick={handleExport} disabled={exporting}>
            <Download className="h-4 w-4 mr-2" />
            {exporting ? "Exporting..." : "Export CSV"}
          </Button>
          <Button onClick={openCreate}>
            <Plus className="h-4 w-4 mr-2" />
            Add Webhook
          </Button>
        </div>
      </div>

      {error && (
        <ErrorAlert message={error} onRetry={load} onDismiss={() => setError("")} className="mb-4" />
      )}

      {/* Search bar */}
      <div className="mb-4 flex gap-2">
        <div className="relative flex-1 max-w-sm">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
          <Input
            placeholder="Search by URL or description..."
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
      </div>

      {/* Secret reveal after creation */}
      {createdWebhook && (
        <div className="mb-4 p-4 bg-green-50 border border-green-200 dark:bg-green-500/10 dark:border-green-500/20 rounded-md">
          <div className="flex items-center justify-between mb-2">
            <p className="text-sm font-medium text-green-800 dark:text-green-400">
              Webhook created — signing secret:
            </p>
            <Button
              variant="ghost"
              size="sm"
              onClick={() => setCreatedWebhook(null)}
            >
              <span className="sr-only">Dismiss</span>
              &times;
            </Button>
          </div>
          <p className="text-xs text-green-700 dark:text-green-400 mb-2">
            Copy this secret now. Use it to verify webhook signatures.
          </p>
          <div className="flex items-center gap-2">
            <code className="flex-1 bg-background border rounded px-3 py-2 text-sm font-mono break-all">
              {createdWebhook.secret}
            </code>
            <Button
              variant="outline"
              size="sm"
              onClick={() => copySecret(createdWebhook.secret)}
            >
              {copied ? (
                <Check className="h-4 w-4 text-green-600 dark:text-green-500" />
              ) : (
                <Copy className="h-4 w-4" />
              )}
            </Button>
          </div>
        </div>
      )}

      <div className="border rounded-lg overflow-hidden">
        <table className="w-full text-sm">
          <thead>
            <tr className="bg-muted/50 border-b">
              <SortableHeader label="URL" column="url" sort={sort} onSort={toggleSort} />
              <th className="text-left p-3 font-medium hidden md:table-cell">Description</th>
              <th className="text-left p-3 font-medium hidden lg:table-cell">Events</th>
              <SortableHeader label="Status" column="is_active" sort={sort} onSort={toggleSort} />
              <SortableHeader label="Created" column="created_at" sort={sort} onSort={toggleSort} className="hidden md:table-cell" />
              <th className="text-right p-3 font-medium">Actions</th>
            </tr>
          </thead>
          <tbody>
            {webhooks.length === 0 ? (
              <TableEmptyRow colSpan={6} message={searchInput ? "No webhooks match your search" : "No webhooks configured"} />
            ) : (
              webhooks.map((wh) => (
                <tr
                  key={wh.id}
                  className="border-b last:border-0 hover:bg-muted/30 cursor-pointer"
                  onClick={() => navigate(`/webhooks/${wh.id}`)}
                >
                  <td className="p-3">
                    <div className="flex items-center gap-1.5">
                      <span className="font-mono text-xs truncate max-w-[280px]">{wh.url}</span>
                      <ExternalLink className="h-3 w-3 text-muted-foreground shrink-0" />
                    </div>
                  </td>
                  <td className="p-3 text-muted-foreground text-xs hidden md:table-cell">
                    {wh.description || "\u2014"}
                  </td>
                  <td className="p-3 hidden lg:table-cell">
                    <div className="flex flex-wrap gap-1">
                      {wh.events.map((ev) => (
                        <EventBadge key={ev} event={ev} />
                      ))}
                    </div>
                  </td>
                  <td className="p-3">
                    <StatusBadge active={wh.is_active} />
                  </td>
                  <td className="p-3 text-muted-foreground text-xs hidden md:table-cell">
                    {formatTime(wh.created_at)}
                  </td>
                  <td className="p-3 text-right" onClick={(e) => e.stopPropagation()}>
                    <span className="inline-flex items-center gap-1">
                      <Button variant="ghost" size="sm" onClick={() => openEdit(wh)}>
                        <Pencil className="h-3.5 w-3.5" />
                      </Button>
                      <Button variant="ghost" size="sm" onClick={() => setDeleteTarget(wh)}>
                        <Trash2 className="h-3.5 w-3.5 text-red-500" />
                      </Button>
                    </span>
                  </td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>

      <Pagination
        page={page}
        totalPages={totalPages}
        total={total}
        pageSize={pageSize}
        onPrev={() => setPage(page - 1)}
        onNext={() => setPage(page + 1)}
      />

      {/* Create / Edit Dialog */}
      <Dialog open={dialogOpen} onClose={closeDialog}>
        <DialogHeader>
          <DialogTitle>{editing ? "Edit Webhook" : "Add Webhook"}</DialogTitle>
          <DialogCloseButton onClick={closeDialog} />
        </DialogHeader>

        <form onSubmit={handleSubmit}>
          <DialogBody className="space-y-4">
            {formError && (
              <div className="p-3 bg-red-50 border border-red-200 text-red-700 dark:bg-red-500/10 dark:border-red-500/20 dark:text-red-400 rounded-md text-sm">
                {formError}
              </div>
            )}

            <div className="space-y-2">
              <Label htmlFor="url">Endpoint URL</Label>
              <Input
                id="url"
                value={url}
                onChange={(e) => setUrl(e.target.value)}
                placeholder="https://example.com/webhook"
              />
            </div>

            <div className="space-y-2">
              <Label htmlFor="description">
                Description{" "}
                <span className="text-muted-foreground font-normal">(optional)</span>
              </Label>
              <Input
                id="description"
                value={description}
                onChange={(e) => setDescription(e.target.value)}
                placeholder="e.g. Slack notification for new users"
              />
            </div>

            <div className="space-y-2">
              <Label htmlFor="events">
                Events{" "}
                <span className="text-muted-foreground font-normal">
                  (comma-separated, * = all)
                </span>
              </Label>
              <Input
                id="events"
                value={events}
                onChange={(e) => setEvents(e.target.value)}
                placeholder="e.g. user.created, user.updated"
              />
            </div>

            {editing && (
              <div className="flex items-center gap-2">
                <input
                  type="checkbox"
                  id="is_active"
                  checked={isActive}
                  onChange={(e) => setIsActive(e.target.checked)}
                  className="h-4 w-4 rounded border-input"
                />
                <Label htmlFor="is_active">Active</Label>
              </div>
            )}
          </DialogBody>

          <DialogFooter>
            <Button type="button" variant="outline" onClick={closeDialog}>
              Cancel
            </Button>
            <Button type="submit" disabled={submitting}>
              {submitting ? "Saving..." : editing ? "Save Changes" : "Add Webhook"}
            </Button>
          </DialogFooter>
        </form>
      </Dialog>

      {/* Delete Confirmation */}
      <ConfirmDialog
        open={!!deleteTarget}
        onClose={() => setDeleteTarget(null)}
        onConfirm={handleDelete}
        title="Delete Webhook"
        message="Are you sure you want to delete this webhook? All delivery history will also be removed."
        confirmLabel="Delete"
        loading={acting === deleteTarget?.id}
        details={deleteTarget && (
          <>
            <div><span className="font-medium">URL:</span> <span className="font-mono text-xs">{deleteTarget.url}</span></div>
            {deleteTarget.description && <div><span className="font-medium">Description:</span> {deleteTarget.description}</div>}
          </>
        )}
      />
    </div>
  );
}

function EventBadge({ event }: { event: string }) {
  const isWildcard = event === "*";
  return (
    <span className={`inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium ${
      isWildcard
        ? "bg-purple-100 text-purple-700 dark:bg-purple-500/10 dark:text-purple-400"
        : "bg-blue-100 text-blue-700 dark:bg-blue-500/10 dark:text-blue-400"
    }`}>
      {isWildcard ? "all events" : event}
    </span>
  );
}

function StatusBadge({ active }: { active: boolean }) {
  if (active) {
    return (
      <span className="inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium bg-green-100 text-green-700 dark:bg-green-500/10 dark:text-green-400">
        Active
      </span>
    );
  }
  return (
    <span className="inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium bg-yellow-100 text-yellow-700 dark:bg-yellow-500/10 dark:text-yellow-400">
      Paused
    </span>
  );
}
