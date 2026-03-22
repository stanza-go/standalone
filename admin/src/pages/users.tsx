import { useCallback, useEffect, useState } from "react";
import { useNavigate } from "react-router";
import { toast } from "sonner";
import { get, post, put, del, downloadCSV, ApiError } from "@/lib/api";
import { useDebounce } from "@/lib/use-debounce";
import { useSelection } from "@/lib/use-selection";
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
import { BulkActionBar } from "@/components/ui/bulk-action-bar";
import {
  Plus,
  Pencil,
  Trash2,
  Search,
  UserCheck,
  Copy,
  Check,
  X,
  Download,
} from "lucide-react";
import { TableSkeleton } from "@/components/ui/skeleton";
import { ErrorAlert } from "@/components/ui/error-alert";
import { Pagination } from "@/components/ui/pagination";
import { TableEmptyRow } from "@/components/ui/empty-state";
import { SortableHeader, useSort } from "@/components/ui/sortable-header";

interface User {
  id: number;
  email: string;
  name: string;
  is_active: boolean;
  created_at: string;
  updated_at: string;
}

export default function UsersPage() {
  const navigate = useNavigate();
  const [users, setUsers] = useState<User[]>([]);
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

  // Sort.
  const [sort, toggleSort] = useSort("id", "desc");

  // Selection.
  const selection = useSelection<number>();
  const [bulkDeleting, setBulkDeleting] = useState(false);
  const [bulkConfirmOpen, setBulkConfirmOpen] = useState(false);

  // Dialog state.
  const [dialogOpen, setDialogOpen] = useState(false);
  const [editing, setEditing] = useState<User | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<User | null>(null);

  // Impersonate state.
  const [impersonateToken, setImpersonateToken] = useState<string | null>(null);
  const [copied, setCopied] = useState(false);

  // Form state.
  const [email, setEmail] = useState("");
  const [name, setName] = useState("");
  const [password, setPassword] = useState("");
  const [formError, setFormError] = useState("");
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({});
  const [submitting, setSubmitting] = useState(false);
  const [exporting, setExporting] = useState(false);

  async function handleExport() {
    setExporting(true);
    try {
      const params = new URLSearchParams();
      if (search) params.set("search", search);
      params.set("sort", sort.column);
      params.set("order", sort.direction);
      await downloadCSV(`/admin/users/export?${params}`);
    } catch {
      toast.error("Failed to export users");
    } finally {
      setExporting(false);
    }
  }

  const load = useCallback(async () => {
    try {
      const params = new URLSearchParams();
      params.set("limit", String(pageSize));
      params.set("offset", String(page * pageSize));
      if (search) params.set("search", search);
      params.set("sort", sort.column);
      params.set("order", sort.direction);

      const data = await get<{ users: User[]; total: number }>(
        `/admin/users?${params}`
      );
      setUsers(data.users);
      setTotal(data.total);
      setError("");
    } catch (e: any) {
      setError(e.message || "Failed to load users");
    } finally {
      setLoading(false);
    }
  }, [page, search, sort.column, sort.direction]);

  useEffect(() => {
    load();
  }, [load]);

  // Reset to first page when search changes.
  useEffect(() => {
    setPage(0);
  }, [search]);

  // Clear selection when page, search, or sort changes.
  useEffect(() => {
    selection.clear();
  }, [page, search, sort.column, sort.direction]);

  function openCreate() {
    setEditing(null);
    setEmail("");
    setName("");
    setPassword("");
    setFormError("");
    setFieldErrors({});
    setDialogOpen(true);
  }

  function openEdit(user: User) {
    setEditing(user);
    setEmail(user.email);
    setName(user.name);
    setPassword("");
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
        const body: Record<string, unknown> = { name };
        if (password) body.password = password;
        await put(`/admin/users/${editing.id}`, body);
        toast.success("User updated");
      } else {
        await post("/admin/users", { email, password, name });
        toast.success("User created");
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

  async function handleDelete() {
    if (!deleteTarget) return;
    const id = deleteTarget.id;
    setActing(id);
    try {
      await del(`/admin/users/${id}`);
      setDeleteTarget(null);
      toast.success("User deleted");
      await load();
    } catch (e: any) {
      toast.error(e.message || "Failed to delete user");
    } finally {
      setActing(null);
    }
  }

  async function handleToggleActive(user: User) {
    setActing(user.id);
    try {
      await put(`/admin/users/${user.id}`, { is_active: !user.is_active });
      toast.success(user.is_active ? "User deactivated" : "User activated");
      await load();
    } catch (e: any) {
      toast.error(e.message || "Failed to update user");
    } finally {
      setActing(null);
    }
  }

  async function handleImpersonate(id: number) {
    setActing(id);
    try {
      const data = await post<{ token: string }>(`/admin/users/${id}/impersonate`);
      setImpersonateToken(data.token);
      setCopied(false);
    } catch (e: any) {
      setError(e.message || "Failed to impersonate user");
    } finally {
      setActing(null);
    }
  }

  async function handleBulkDelete() {
    setBulkDeleting(true);
    try {
      const data = await post<{ affected: number }>("/admin/users/bulk-delete", { ids: selection.ids });
      setBulkConfirmOpen(false);
      selection.clear();
      toast.success(`${data.affected} user${data.affected !== 1 ? "s" : ""} deleted`);
      await load();
    } catch (e: any) {
      toast.error(e.message || "Failed to bulk delete users");
    } finally {
      setBulkDeleting(false);
    }
  }

  async function copyToken() {
    if (!impersonateToken) return;
    await navigator.clipboard.writeText(impersonateToken);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
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
        <h1 className="text-2xl font-bold mb-6">Users</h1>
        <TableSkeleton columns={[
          { width: "w-10", hidden: "hidden md:table-cell" },
          { width: "w-32" },
          { width: "w-24" },
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
          <h1 className="text-2xl font-bold">Users</h1>
          <p className="text-sm text-muted-foreground mt-1">
            {total} user{total !== 1 ? "s" : ""}
          </p>
        </div>
        <div className="flex gap-2">
          <Button variant="outline" onClick={handleExport} disabled={exporting}>
            <Download className="h-4 w-4 mr-2" />
            {exporting ? "Exporting..." : "Export CSV"}
          </Button>
          <Button onClick={openCreate}>
            <Plus className="h-4 w-4 mr-2" />
            Create User
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
            placeholder="Search by email or name..."
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

      <div className="border rounded-lg overflow-hidden">
        <table className="w-full text-sm">
          <thead>
            <tr className="bg-muted/50 border-b">
              <th className="p-3 w-10">
                <input
                  type="checkbox"
                  checked={selection.isAllSelected(users.map((u) => u.id))}
                  onChange={() => selection.toggleAll(users.map((u) => u.id))}
                  className="rounded border-input"
                />
              </th>
              <SortableHeader label="ID" column="id" sort={sort} onSort={toggleSort} className="hidden md:table-cell" />
              <SortableHeader label="Email" column="email" sort={sort} onSort={toggleSort} />
              <SortableHeader label="Name" column="name" sort={sort} onSort={toggleSort} />
              <SortableHeader label="Status" column="is_active" sort={sort} onSort={toggleSort} />
              <SortableHeader label="Created" column="created_at" sort={sort} onSort={toggleSort} className="hidden md:table-cell" />
              <th className="text-right p-3 font-medium">Actions</th>
            </tr>
          </thead>
          <tbody>
            {users.length === 0 ? (
              <TableEmptyRow colSpan={7} message={search ? "No users match your search" : "No users found"} />
            ) : (
              users.map((user) => (
                <tr
                  key={user.id}
                  className={`border-b last:border-0 hover:bg-muted/30 ${selection.isSelected(user.id) ? "bg-muted/40" : ""}`}
                >
                  <td className="p-3">
                    <input
                      type="checkbox"
                      checked={selection.isSelected(user.id)}
                      onChange={() => selection.toggle(user.id)}
                      className="rounded border-input"
                    />
                  </td>
                  <td className="p-3 font-mono text-xs hidden md:table-cell">{user.id}</td>
                  <td className="p-3">
                    <button
                      onClick={() => navigate(`/users/${user.id}`)}
                      className="hover:underline text-left"
                    >
                      {user.email}
                    </button>
                  </td>
                  <td className="p-3">{user.name || "\u2014"}</td>
                  <td className="p-3">
                    <button
                      onClick={() => handleToggleActive(user)}
                      disabled={acting === user.id}
                      className="cursor-pointer"
                    >
                      <StatusBadge active={user.is_active} />
                    </button>
                  </td>
                  <td className="p-3 text-muted-foreground text-xs hidden md:table-cell">
                    {formatTime(user.created_at)}
                  </td>
                  <td className="p-3 text-right">
                    <span className="inline-flex items-center gap-1">
                      <Button
                        variant="ghost"
                        size="sm"
                        title="Impersonate"
                        disabled={!user.is_active}
                        onClick={() => handleImpersonate(user.id)}
                      >
                        <UserCheck className="h-3.5 w-3.5 text-amber-600 dark:text-amber-500" />
                      </Button>
                      <Button
                        variant="ghost"
                        size="sm"
                        onClick={() => openEdit(user)}
                      >
                        <Pencil className="h-3.5 w-3.5" />
                      </Button>
                      <Button
                        variant="ghost"
                        size="sm"
                        onClick={() => setDeleteTarget(user)}
                      >
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
          <DialogTitle>{editing ? "Edit User" : "Create User"}</DialogTitle>
          <DialogCloseButton onClick={closeDialog} />
        </DialogHeader>

        <form onSubmit={handleSubmit}>
          <DialogBody className="space-y-4">
            {formError && !Object.keys(fieldErrors).length && (
              <div className="p-3 bg-red-50 border border-red-200 text-red-700 dark:bg-red-500/10 dark:border-red-500/20 dark:text-red-400 rounded-md text-sm">
                {formError}
              </div>
            )}

            <div className="space-y-2">
              <Label htmlFor="email">Email</Label>
              <Input
                id="email"
                type="email"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                disabled={!!editing}
                placeholder="user@example.com"
                className={fieldErrors.email ? "border-destructive" : ""}
              />
              {fieldErrors.email && (
                <p className="text-sm text-destructive">{fieldErrors.email}</p>
              )}
            </div>

            <div className="space-y-2">
              <Label htmlFor="name">Name</Label>
              <Input
                id="name"
                value={name}
                onChange={(e) => setName(e.target.value)}
                placeholder="Full name"
                className={fieldErrors.name ? "border-destructive" : ""}
              />
              {fieldErrors.name && (
                <p className="text-sm text-destructive">{fieldErrors.name}</p>
              )}
            </div>

            <div className="space-y-2">
              <Label htmlFor="password">
                {editing
                  ? "New Password (leave empty to keep current)"
                  : "Password"}
              </Label>
              <Input
                id="password"
                type="password"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                placeholder={
                  editing ? "Leave empty to keep current" : "Enter password"
                }
                className={fieldErrors.password ? "border-destructive" : ""}
              />
              {fieldErrors.password && (
                <p className="text-sm text-destructive">{fieldErrors.password}</p>
              )}
            </div>
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
                  : "Create User"}
            </Button>
          </DialogFooter>
        </form>
      </Dialog>

      {/* Impersonate Token Dialog */}
      <Dialog
        open={!!impersonateToken}
        onClose={() => setImpersonateToken(null)}
        className="[&>div]:max-w-lg"
      >
        <DialogHeader>
          <DialogTitle>Impersonation Token</DialogTitle>
          <DialogCloseButton onClick={() => setImpersonateToken(null)} />
        </DialogHeader>

        <DialogBody className="space-y-3">
          <p className="text-sm text-muted-foreground">
            This is a short-lived access token for the selected user. Use it
            as a Bearer token or in an Authorization header for debugging.
          </p>
          <div className="relative">
            <pre className="p-3 bg-muted rounded-md text-xs break-all whitespace-pre-wrap font-mono">
              {impersonateToken}
            </pre>
            <Button
              variant="outline"
              size="sm"
              className="absolute top-2 right-2"
              onClick={copyToken}
            >
              {copied ? (
                <Check className="h-3.5 w-3.5 text-green-600 dark:text-green-500" />
              ) : (
                <Copy className="h-3.5 w-3.5" />
              )}
            </Button>
          </div>
        </DialogBody>

        <DialogFooter>
          <Button variant="outline" onClick={() => setImpersonateToken(null)}>
            Close
          </Button>
        </DialogFooter>
      </Dialog>

      {/* Delete Confirmation */}
      <ConfirmDialog
        open={!!deleteTarget}
        onClose={() => setDeleteTarget(null)}
        onConfirm={handleDelete}
        title="Delete User"
        message="Are you sure you want to delete this user? This action cannot be undone."
        confirmLabel="Delete"
        loading={acting === deleteTarget?.id}
        details={deleteTarget && (
          <>
            <div><span className="font-medium">Email:</span> {deleteTarget.email}</div>
            <div><span className="font-medium">Name:</span> {deleteTarget.name || "\u2014"}</div>
          </>
        )}
      />

      {/* Bulk Actions */}
      <BulkActionBar count={selection.count} onClear={selection.clear}>
        <Button variant="destructive" size="sm" onClick={() => setBulkConfirmOpen(true)}>
          <Trash2 className="h-3.5 w-3.5 mr-1" />
          Delete
        </Button>
      </BulkActionBar>

      <ConfirmDialog
        open={bulkConfirmOpen}
        onClose={() => setBulkConfirmOpen(false)}
        onConfirm={handleBulkDelete}
        title="Delete Users"
        message={`Are you sure you want to delete ${selection.count} user${selection.count !== 1 ? "s" : ""}? This action cannot be undone.`}
        confirmLabel="Delete"
        loading={bulkDeleting}
      />
    </div>
  );
}

function StatusBadge({ active }: { active: boolean }) {
  return (
    <span
      className={`inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium ${
        active ? "bg-green-100 text-green-700 dark:bg-green-500/10 dark:text-green-400" : "bg-red-100 text-red-700 dark:bg-red-500/10 dark:text-red-400"
      }`}
    >
      {active ? "Active" : "Inactive"}
    </span>
  );
}
