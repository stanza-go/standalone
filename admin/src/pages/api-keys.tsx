import { useCallback, useEffect, useState } from "react";
import { toast } from "sonner";
import { get, post, put, del, ApiError } from "@/lib/api";
import { useDebounce } from "@/lib/use-debounce";
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
import { Plus, Pencil, Trash2, Copy, Check, Search, X } from "lucide-react";
import { TableSkeleton } from "@/components/ui/skeleton";
import { ErrorAlert } from "@/components/ui/error-alert";
import { Pagination } from "@/components/ui/pagination";
import { TableEmptyRow } from "@/components/ui/empty-state";

interface APIKey {
  id: number;
  name: string;
  key_prefix: string;
  scopes: string;
  created_by: number;
  request_count: number;
  last_used_at: string;
  expires_at: string;
  created_at: string;
  revoked_at: string;
}

interface CreatedKey {
  id: number;
  name: string;
  key: string;
  key_prefix: string;
  scopes: string;
  created_by: number;
  expires_at: string;
  created_at: string;
}

export default function APIKeysPage() {
  const [keys, setKeys] = useState<APIKey[]>([]);
  const [total, setTotal] = useState(0);
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(true);
  const [acting, setActing] = useState<number | null>(null);

  // Pagination.
  const [page, setPage] = useState(0);
  const pageSize = 20;

  // Search.
  const [searchInput, setSearchInput] = useState("");
  const search = useDebounce(searchInput, 300);

  // Dialog state.
  const [dialogOpen, setDialogOpen] = useState(false);
  const [editing, setEditing] = useState<APIKey | null>(null);
  const [revokeId, setRevokeId] = useState<number | null>(null);

  // Form state.
  const [name, setName] = useState("");
  const [scopes, setScopes] = useState("");
  const [expiresAt, setExpiresAt] = useState("");
  const [formError, setFormError] = useState("");
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({});
  const [submitting, setSubmitting] = useState(false);

  // Created key reveal state.
  const [createdKey, setCreatedKey] = useState<CreatedKey | null>(null);
  const [copied, setCopied] = useState(false);

  const load = useCallback(async () => {
    try {
      const params = new URLSearchParams();
      params.set("limit", String(pageSize));
      params.set("offset", String(page * pageSize));
      if (search) params.set("search", search);

      const data = await get<{ api_keys: APIKey[]; total: number }>(
        `/admin/api-keys?${params}`
      );
      setKeys(data.api_keys);
      setTotal(data.total);
      setError("");
    } catch (e: any) {
      setError(e.message || "Failed to load API keys");
    } finally {
      setLoading(false);
    }
  }, [page, search]);

  useEffect(() => {
    load();
    const interval = setInterval(load, 30000);
    return () => clearInterval(interval);
  }, [load]);

  // Reset to first page when search changes.
  useEffect(() => {
    setPage(0);
  }, [search]);

  function openCreate() {
    setEditing(null);
    setName("");
    setScopes("");
    setExpiresAt("");
    setFormError("");
    setFieldErrors({});
    setDialogOpen(true);
  }

  function openEdit(key: APIKey) {
    setEditing(key);
    setName(key.name);
    setScopes(key.scopes);
    setExpiresAt("");
    setFormError("");
    setFieldErrors({});
    setDialogOpen(true);
  }

  function closeDialog() {
    setDialogOpen(false);
    setEditing(null);
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setFormError("");
    setFieldErrors({});
    setSubmitting(true);

    try {
      if (editing) {
        await put(`/admin/api-keys/${editing.id}`, { name, scopes });
        toast.success("API key updated");
      } else {
        const body: Record<string, unknown> = { name, scopes };
        if (expiresAt) {
          body.expires_at = new Date(expiresAt).toISOString().replace(/\.\d{3}Z$/, "Z");
        }
        const data = await post<{ api_key: CreatedKey }>("/admin/api-keys", body);
        setCreatedKey(data.api_key);
        setCopied(false);
        toast.success("API key created");
      }
      closeDialog();
      await load();
    } catch (err) {
      if (err instanceof ApiError) {
        setFormError(err.message);
        setFieldErrors(err.fields);
      } else {
        setFormError("Operation failed");
      }
    } finally {
      setSubmitting(false);
    }
  }

  async function handleRevoke(id: number) {
    setActing(id);
    try {
      await del(`/admin/api-keys/${id}`);
      setRevokeId(null);
      toast.success("API key revoked");
      await load();
    } catch (e: any) {
      toast.error(e.message || "Failed to revoke key");
    } finally {
      setActing(null);
    }
  }

  async function copyKey(key: string) {
    await navigator.clipboard.writeText(key);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  }

  function formatTime(iso: string): string {
    if (!iso) return "\u2014";
    const d = new Date(iso);
    return d.toLocaleDateString() + " " + d.toLocaleTimeString();
  }

  function isExpired(expiresAt: string): boolean {
    if (!expiresAt) return false;
    return new Date(expiresAt) < new Date();
  }

  const totalPages = Math.ceil(total / pageSize);

  if (loading) {
    return (
      <div className="p-6">
        <h1 className="text-2xl font-bold mb-6">API Keys</h1>
        <TableSkeleton columns={[
          { width: "w-24" },
          { width: "w-20", hidden: "hidden md:table-cell" },
          { width: "w-24", hidden: "hidden lg:table-cell" },
          { width: "w-20", hidden: "hidden lg:table-cell" },
          { width: "w-14", hidden: "hidden lg:table-cell" },
          { width: "w-20", hidden: "hidden md:table-cell" },
          { width: "w-16" },
          { width: "w-20", hidden: "hidden md:table-cell" },
          { width: "w-20" },
        ]} />
      </div>
    );
  }

  return (
    <div className="p-6">
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-2xl font-bold">API Keys</h1>
          <p className="text-sm text-muted-foreground mt-1">
            {total} key{total !== 1 ? "s" : ""}
          </p>
        </div>
        <Button onClick={openCreate}>
          <Plus className="h-4 w-4 mr-2" />
          Create Key
        </Button>
      </div>

      {error && (
        <ErrorAlert message={error} onRetry={load} onDismiss={() => setError("")} className="mb-4" />
      )}

      {/* Search bar */}
      <div className="mb-4 flex gap-2">
        <div className="relative flex-1 max-w-sm">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
          <Input
            placeholder="Search by name or key prefix..."
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

      {/* Key reveal after creation */}
      {createdKey && (
        <div className="mb-4 p-4 bg-green-50 border border-green-200 rounded-md">
          <div className="flex items-center justify-between mb-2">
            <p className="text-sm font-medium text-green-800">
              API key created: {createdKey.name}
            </p>
            <Button
              variant="ghost"
              size="sm"
              onClick={() => setCreatedKey(null)}
            >
              <span className="sr-only">Dismiss</span>
              &times;
            </Button>
          </div>
          <p className="text-xs text-green-700 mb-2">
            Copy this key now. It will not be shown again.
          </p>
          <div className="flex items-center gap-2">
            <code className="flex-1 bg-white border rounded px-3 py-2 text-sm font-mono break-all">
              {createdKey.key}
            </code>
            <Button
              variant="outline"
              size="sm"
              onClick={() => copyKey(createdKey.key)}
            >
              {copied ? (
                <Check className="h-4 w-4 text-green-600" />
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
              <th className="text-left p-3 font-medium">Name</th>
              <th className="text-left p-3 font-medium hidden md:table-cell">Key</th>
              <th className="text-left p-3 font-medium hidden lg:table-cell">Scopes</th>
              <th className="text-left p-3 font-medium hidden lg:table-cell">Last Used</th>
              <th className="text-right p-3 font-medium hidden lg:table-cell">Requests</th>
              <th className="text-left p-3 font-medium hidden md:table-cell">Expires</th>
              <th className="text-left p-3 font-medium">Status</th>
              <th className="text-left p-3 font-medium hidden md:table-cell">Created</th>
              <th className="text-right p-3 font-medium">Actions</th>
            </tr>
          </thead>
          <tbody>
            {keys.length === 0 ? (
              <TableEmptyRow colSpan={9} message={search ? "No API keys match your search" : "No API keys found"} />
            ) : (
              keys.map((k) => (
                <tr
                  key={k.id}
                  className="border-b last:border-0 hover:bg-muted/30"
                >
                  <td className="p-3 font-medium">{k.name}</td>
                  <td className="p-3 font-mono text-xs text-muted-foreground hidden md:table-cell">
                    {k.key_prefix}...
                  </td>
                  <td className="p-3 hidden lg:table-cell">
                    {k.scopes ? (
                      <div className="flex flex-wrap gap-1">
                        {k.scopes.split(",").map((s) => (
                          <ScopeBadge key={s} scope={s.trim()} />
                        ))}
                      </div>
                    ) : (
                      <span className="text-muted-foreground text-xs">
                        all
                      </span>
                    )}
                  </td>
                  <td className="p-3 text-muted-foreground text-xs hidden lg:table-cell">
                    {formatTime(k.last_used_at)}
                  </td>
                  <td className="p-3 text-right text-xs font-mono hidden lg:table-cell">
                    {k.request_count > 0 ? k.request_count.toLocaleString() : "\u2014"}
                  </td>
                  <td className="p-3 text-muted-foreground text-xs hidden md:table-cell">
                    {k.expires_at ? (
                      <span
                        className={
                          isExpired(k.expires_at) ? "text-red-600" : ""
                        }
                      >
                        {formatTime(k.expires_at)}
                      </span>
                    ) : (
                      "Never"
                    )}
                  </td>
                  <td className="p-3">
                    <KeyStatusBadge
                      revoked={!!k.revoked_at}
                      expired={isExpired(k.expires_at)}
                    />
                  </td>
                  <td className="p-3 text-muted-foreground text-xs hidden md:table-cell">
                    {formatTime(k.created_at)}
                  </td>
                  <td className="p-3 text-right">
                    {k.revoked_at ? (
                      <span className="text-xs text-muted-foreground">
                        Revoked
                      </span>
                    ) : revokeId === k.id ? (
                      <span className="inline-flex items-center gap-2">
                        <span className="text-xs text-muted-foreground">
                          Revoke?
                        </span>
                        <Button
                          variant="destructive"
                          size="sm"
                          disabled={acting === k.id}
                          onClick={() => handleRevoke(k.id)}
                        >
                          {acting === k.id ? "Revoking..." : "Confirm"}
                        </Button>
                        <Button
                          variant="ghost"
                          size="sm"
                          onClick={() => setRevokeId(null)}
                        >
                          Cancel
                        </Button>
                      </span>
                    ) : (
                      <span className="inline-flex items-center gap-1">
                        <Button
                          variant="ghost"
                          size="sm"
                          onClick={() => openEdit(k)}
                        >
                          <Pencil className="h-3.5 w-3.5" />
                        </Button>
                        <Button
                          variant="ghost"
                          size="sm"
                          onClick={() => setRevokeId(k.id)}
                        >
                          <Trash2 className="h-3.5 w-3.5 text-red-500" />
                        </Button>
                      </span>
                    )}
                  </td>
                </tr>
              ))
            )}
          </tbody>
        </table>
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

      {/* Create / Edit Dialog */}
      <Dialog open={dialogOpen} onClose={closeDialog}>
        <DialogHeader>
          <DialogTitle>{editing ? "Edit API Key" : "Create API Key"}</DialogTitle>
          <DialogCloseButton onClick={closeDialog} />
        </DialogHeader>

        <form onSubmit={handleSubmit}>
          <DialogBody className="space-y-4">
            {formError && !Object.keys(fieldErrors).length && (
              <div className="p-3 bg-red-50 border border-red-200 text-red-700 rounded-md text-sm">
                {formError}
              </div>
            )}

            <div className="space-y-2">
              <Label htmlFor="name">Name</Label>
              <Input
                id="name"
                value={name}
                onChange={(e) => setName(e.target.value)}
                placeholder="e.g. Production API"
                className={fieldErrors.name ? "border-destructive" : ""}
              />
              {fieldErrors.name && (
                <p className="text-sm text-destructive">{fieldErrors.name}</p>
              )}
            </div>

            <div className="space-y-2">
              <Label htmlFor="scopes">
                Scopes{" "}
                <span className="text-muted-foreground font-normal">
                  (comma-separated, empty = all)
                </span>
              </Label>
              <Input
                id="scopes"
                value={scopes}
                onChange={(e) => setScopes(e.target.value)}
                placeholder="e.g. read,write"
              />
            </div>

            {!editing && (
              <div className="space-y-2">
                <Label htmlFor="expires">
                  Expires{" "}
                  <span className="text-muted-foreground font-normal">
                    (optional)
                  </span>
                </Label>
                <Input
                  id="expires"
                  type="datetime-local"
                  value={expiresAt}
                  onChange={(e) => setExpiresAt(e.target.value)}
                />
              </div>
            )}
          </DialogBody>

          <DialogFooter>
            <Button type="button" variant="outline" onClick={closeDialog}>
              Cancel
            </Button>
            <Button type="submit" disabled={submitting}>
              {submitting
                ? "Saving..."
                : editing
                  ? "Save Changes"
                  : "Create Key"}
            </Button>
          </DialogFooter>
        </form>
      </Dialog>
    </div>
  );
}

function ScopeBadge({ scope }: { scope: string }) {
  return (
    <span className="inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium bg-blue-100 text-blue-700">
      {scope}
    </span>
  );
}

function KeyStatusBadge({
  revoked,
  expired,
}: {
  revoked: boolean;
  expired: boolean;
}) {
  if (revoked) {
    return (
      <span className="inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium bg-red-100 text-red-700">
        Revoked
      </span>
    );
  }
  if (expired) {
    return (
      <span className="inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium bg-yellow-100 text-yellow-700">
        Expired
      </span>
    );
  }
  return (
    <span className="inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium bg-green-100 text-green-700">
      Active
    </span>
  );
}
