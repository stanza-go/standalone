import { useCallback, useEffect, useState } from "react";
import { useNavigate } from "react-router";
import { toast } from "sonner";
import { get, post, put, del, ApiError, downloadCSV } from "@/lib/api";
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
import { Plus, Pencil, Trash2, Search, X, Download } from "lucide-react";
import { TableSkeleton } from "@/components/ui/skeleton";
import { ErrorAlert } from "@/components/ui/error-alert";
import { Pagination } from "@/components/ui/pagination";
import { TableEmptyRow } from "@/components/ui/empty-state";
import { SortableHeader, useSort } from "@/components/ui/sortable-header";

interface Admin {
  id: number;
  email: string;
  name: string;
  role: string;
  is_active: boolean;
  created_at: string;
  updated_at: string;
}

export default function AdminsPage() {
  const navigate = useNavigate();
  const [admins, setAdmins] = useState<Admin[]>([]);
  const [total, setTotal] = useState(0);
  const [availableRoles, setAvailableRoles] = useState<string[]>([]);
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(true);
  const [acting, setActing] = useState<number | null>(null);
  const [exporting, setExporting] = useState(false);

  // Pagination.
  const [page, setPage] = useState(0);
  const pageSize = 20;

  // Search.
  const [searchInput, setSearchInput] = useState("");
  const search = useDebounce(searchInput, 300);

  // Sort.
  const [sort, toggleSort] = useSort("id", "asc");

  // Selection.
  const selection = useSelection<number>();
  const [bulkDeleting, setBulkDeleting] = useState(false);
  const [bulkConfirmOpen, setBulkConfirmOpen] = useState(false);

  // Dialog state.
  const [dialogOpen, setDialogOpen] = useState(false);
  const [editing, setEditing] = useState<Admin | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<Admin | null>(null);

  // Form state.
  const [email, setEmail] = useState("");
  const [name, setName] = useState("");
  const [role, setRole] = useState("admin");
  const [password, setPassword] = useState("");
  const [formError, setFormError] = useState("");
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({});
  const [submitting, setSubmitting] = useState(false);

  const load = useCallback(async () => {
    try {
      const params = new URLSearchParams();
      params.set("limit", String(pageSize));
      params.set("offset", String(page * pageSize));
      if (search) params.set("search", search);
      params.set("sort", sort.column);
      params.set("order", sort.direction);

      const [adminsData, rolesData] = await Promise.all([
        get<{ admins: Admin[]; total: number }>(`/admin/admins?${params}`),
        get<{ roles: string[] }>("/admin/role-names"),
      ]);
      setAdmins(adminsData.admins);
      setTotal(adminsData.total);
      setAvailableRoles(rolesData.roles);
      setError("");
    } catch (e: any) {
      setError(e.message || "Failed to load admins");
    } finally {
      setLoading(false);
    }
  }, [page, search, sort.column, sort.direction]);

  useEffect(() => {
    load();
    const interval = setInterval(load, 30000);
    return () => clearInterval(interval);
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
    setRole("admin");
    setPassword("");
    setFormError("");
    setFieldErrors({});
    setDialogOpen(true);
  }

  function openEdit(admin: Admin) {
    setEditing(admin);
    setEmail(admin.email);
    setName(admin.name);
    setRole(admin.role);
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
        const body: Record<string, unknown> = { name, role };
        if (password) body.password = password;
        await put(`/admin/admins/${editing.id}`, body);
        toast.success("Admin updated");
      } else {
        await post("/admin/admins", { email, password, name, role });
        toast.success("Admin created");
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
      await del(`/admin/admins/${id}`);
      setDeleteTarget(null);
      toast.success("Admin deleted");
      await load();
    } catch (e: any) {
      toast.error(e.message || "Failed to delete admin");
    } finally {
      setActing(null);
    }
  }

  async function handleExport() {
    setExporting(true);
    try {
      const params = new URLSearchParams();
      if (search) params.set("search", search);
      params.set("sort", sort.column);
      params.set("order", sort.direction);
      await downloadCSV(`/admin/admins/export?${params}`);
    } catch {
      toast.error("Failed to export admins");
    } finally {
      setExporting(false);
    }
  }

  async function handleBulkDelete() {
    setBulkDeleting(true);
    try {
      const data = await post<{ affected: number }>("/admin/admins/bulk-delete", { ids: selection.ids });
      setBulkConfirmOpen(false);
      selection.clear();
      toast.success(`${data.affected} admin${data.affected !== 1 ? "s" : ""} deleted`);
      await load();
    } catch (e: any) {
      toast.error(e.message || "Failed to bulk delete admins");
    } finally {
      setBulkDeleting(false);
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
        <h1 className="text-2xl font-bold mb-6">Admin Users</h1>
        <TableSkeleton columns={[
          { width: "w-10", hidden: "hidden md:table-cell" },
          { width: "w-32" },
          { width: "w-24", hidden: "hidden md:table-cell" },
          { width: "w-20" },
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
          <h1 className="text-2xl font-bold">Admin Users</h1>
          <p className="text-sm text-muted-foreground mt-1">
            {total} admin{total !== 1 ? "s" : ""}
          </p>
        </div>
        <div className="flex gap-2">
          <Button variant="outline" onClick={handleExport} disabled={exporting}>
            <Download className="h-4 w-4 mr-2" />
            {exporting ? "Exporting..." : "Export CSV"}
          </Button>
          <Button onClick={openCreate}>
            <Plus className="h-4 w-4 mr-2" />
            Create Admin
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
                  checked={selection.isAllSelected(admins.map((a) => a.id))}
                  onChange={() => selection.toggleAll(admins.map((a) => a.id))}
                  className="rounded border-input"
                />
              </th>
              <SortableHeader label="ID" column="id" sort={sort} onSort={toggleSort} className="hidden md:table-cell" />
              <SortableHeader label="Email" column="email" sort={sort} onSort={toggleSort} />
              <SortableHeader label="Name" column="name" sort={sort} onSort={toggleSort} className="hidden md:table-cell" />
              <SortableHeader label="Role" column="role" sort={sort} onSort={toggleSort} />
              <SortableHeader label="Status" column="is_active" sort={sort} onSort={toggleSort} />
              <SortableHeader label="Created" column="created_at" sort={sort} onSort={toggleSort} className="hidden md:table-cell" />
              <th className="text-right p-3 font-medium">Actions</th>
            </tr>
          </thead>
          <tbody>
            {admins.length === 0 ? (
              <TableEmptyRow colSpan={8} message={search ? "No admins match your search" : "No admins found"} />
            ) : (
              admins.map((admin) => (
                <tr
                  key={admin.id}
                  className={`border-b last:border-0 hover:bg-muted/30 ${selection.isSelected(admin.id) ? "bg-muted/40" : ""}`}
                >
                  <td className="p-3">
                    <input
                      type="checkbox"
                      checked={selection.isSelected(admin.id)}
                      onChange={() => selection.toggle(admin.id)}
                      className="rounded border-input"
                    />
                  </td>
                  <td className="p-3 font-mono text-xs hidden md:table-cell">{admin.id}</td>
                  <td className="p-3">
                    <button
                      onClick={() => navigate(`/admins/${admin.id}`)}
                      className="hover:underline text-left"
                    >
                      {admin.email}
                    </button>
                  </td>
                  <td className="p-3 hidden md:table-cell">{admin.name || "\u2014"}</td>
                  <td className="p-3">
                    <RoleBadge role={admin.role} />
                  </td>
                  <td className="p-3">
                    <StatusBadge active={admin.is_active} />
                  </td>
                  <td className="p-3 text-muted-foreground text-xs hidden md:table-cell">
                    {formatTime(admin.created_at)}
                  </td>
                  <td className="p-3 text-right">
                    <span className="inline-flex items-center gap-1">
                      <Button
                        variant="ghost"
                        size="sm"
                        onClick={() => openEdit(admin)}
                      >
                        <Pencil className="h-3.5 w-3.5" />
                      </Button>
                      <Button
                        variant="ghost"
                        size="sm"
                        onClick={() => setDeleteTarget(admin)}
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
          <DialogTitle>{editing ? "Edit Admin" : "Create Admin"}</DialogTitle>
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
                placeholder="admin@example.com"
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
              <Label htmlFor="role">Role</Label>
              <select
                id="role"
                value={role}
                onChange={(e) => setRole(e.target.value)}
                className="flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
              >
                {availableRoles.map((r) => (
                  <option key={r} value={r}>
                    {r}
                  </option>
                ))}
              </select>
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
                  : "Create Admin"}
            </Button>
          </DialogFooter>
        </form>
      </Dialog>

      {/* Delete Confirmation */}
      <ConfirmDialog
        open={!!deleteTarget}
        onClose={() => setDeleteTarget(null)}
        onConfirm={handleDelete}
        title="Delete Admin"
        message="Are you sure you want to delete this admin? They will lose all access immediately."
        confirmLabel="Delete"
        loading={acting === deleteTarget?.id}
        details={deleteTarget && (
          <>
            <div><span className="font-medium">Email:</span> {deleteTarget.email}</div>
            <div><span className="font-medium">Role:</span> {deleteTarget.role}</div>
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
        title="Delete Admins"
        message={`Are you sure you want to delete ${selection.count} admin${selection.count !== 1 ? "s" : ""}? This action cannot be undone.`}
        confirmLabel="Delete"
        loading={bulkDeleting}
      />
    </div>
  );
}

function RoleBadge({ role }: { role: string }) {
  const colors: Record<string, string> = {
    superadmin: "bg-purple-100 text-purple-700 dark:bg-purple-500/10 dark:text-purple-400",
    admin: "bg-blue-100 text-blue-700 dark:bg-blue-500/10 dark:text-blue-400",
    viewer: "bg-gray-100 text-gray-700 dark:bg-gray-500/10 dark:text-gray-400",
  };
  return (
    <span
      className={`inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium ${colors[role] || "bg-indigo-100 text-indigo-700 dark:bg-indigo-500/10 dark:text-indigo-400"}`}
    >
      {role}
    </span>
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
