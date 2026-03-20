import { useCallback, useEffect, useState } from "react";
import { get, post, put, del } from "@/lib/api";
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
import {
  Plus,
  Pencil,
  Trash2,
  Search,
  ChevronLeft,
  ChevronRight,
  UserCheck,
  Copy,
  Check,
} from "lucide-react";

interface User {
  id: number;
  email: string;
  name: string;
  is_active: boolean;
  created_at: string;
  updated_at: string;
}

export default function UsersPage() {
  const [users, setUsers] = useState<User[]>([]);
  const [total, setTotal] = useState(0);
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(true);
  const [acting, setActing] = useState<number | null>(null);

  // Pagination.
  const [page, setPage] = useState(0);
  const pageSize = 20;

  // Search.
  const [search, setSearch] = useState("");
  const [searchInput, setSearchInput] = useState("");

  // Dialog state.
  const [dialogOpen, setDialogOpen] = useState(false);
  const [editing, setEditing] = useState<User | null>(null);
  const [deleteId, setDeleteId] = useState<number | null>(null);

  // Impersonate state.
  const [impersonateToken, setImpersonateToken] = useState<string | null>(null);
  const [copied, setCopied] = useState(false);

  // Form state.
  const [email, setEmail] = useState("");
  const [name, setName] = useState("");
  const [password, setPassword] = useState("");
  const [formError, setFormError] = useState("");
  const [submitting, setSubmitting] = useState(false);

  const load = useCallback(async () => {
    try {
      const params = new URLSearchParams();
      params.set("limit", String(pageSize));
      params.set("offset", String(page * pageSize));
      if (search) params.set("search", search);

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
  }, [page, search]);

  useEffect(() => {
    load();
  }, [load]);

  function handleSearch(e: React.FormEvent) {
    e.preventDefault();
    setPage(0);
    setSearch(searchInput);
  }

  function openCreate() {
    setEditing(null);
    setEmail("");
    setName("");
    setPassword("");
    setFormError("");
    setDialogOpen(true);
  }

  function openEdit(user: User) {
    setEditing(user);
    setEmail(user.email);
    setName(user.name);
    setPassword("");
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

    try {
      if (editing) {
        const body: Record<string, unknown> = { name };
        if (password) body.password = password;
        await put(`/admin/users/${editing.id}`, body);
      } else {
        if (!email || !password) {
          setFormError("Email and password are required");
          setSubmitting(false);
          return;
        }
        await post("/admin/users", { email, password, name });
      }
      closeDialog();
      await load();
    } catch (e: any) {
      setFormError(e.message || "Operation failed");
    } finally {
      setSubmitting(false);
    }
  }

  async function handleDelete(id: number) {
    setActing(id);
    try {
      await del(`/admin/users/${id}`);
      setDeleteId(null);
      await load();
    } catch (e: any) {
      setError(e.message || "Failed to delete user");
    } finally {
      setActing(null);
    }
  }

  async function handleToggleActive(user: User) {
    setActing(user.id);
    try {
      await put(`/admin/users/${user.id}`, { is_active: !user.is_active });
      await load();
    } catch (e: any) {
      setError(e.message || "Failed to update user");
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
        <p className="text-muted-foreground">Loading...</p>
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
        <Button onClick={openCreate}>
          <Plus className="h-4 w-4 mr-2" />
          Create User
        </Button>
      </div>

      {error && (
        <div className="mb-4 p-3 bg-red-50 border border-red-200 text-red-700 rounded-md text-sm">
          {error}
        </div>
      )}

      {/* Search bar */}
      <form onSubmit={handleSearch} className="mb-4 flex gap-2">
        <div className="relative flex-1 max-w-sm">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
          <Input
            placeholder="Search by email or name..."
            value={searchInput}
            onChange={(e) => setSearchInput(e.target.value)}
            className="pl-9"
          />
        </div>
        <Button type="submit" variant="outline">
          Search
        </Button>
        {search && (
          <Button
            type="button"
            variant="ghost"
            onClick={() => {
              setSearchInput("");
              setSearch("");
              setPage(0);
            }}
          >
            Clear
          </Button>
        )}
      </form>

      <div className="border rounded-lg overflow-hidden">
        <table className="w-full text-sm">
          <thead>
            <tr className="bg-muted/50 border-b">
              <th className="text-left p-3 font-medium">ID</th>
              <th className="text-left p-3 font-medium">Email</th>
              <th className="text-left p-3 font-medium">Name</th>
              <th className="text-left p-3 font-medium">Status</th>
              <th className="text-left p-3 font-medium">Created</th>
              <th className="text-right p-3 font-medium">Actions</th>
            </tr>
          </thead>
          <tbody>
            {users.length === 0 ? (
              <tr>
                <td
                  colSpan={6}
                  className="text-center p-6 text-muted-foreground"
                >
                  {search ? "No users match your search" : "No users found"}
                </td>
              </tr>
            ) : (
              users.map((user) => (
                <tr
                  key={user.id}
                  className="border-b last:border-0 hover:bg-muted/30"
                >
                  <td className="p-3 font-mono text-xs">{user.id}</td>
                  <td className="p-3">{user.email}</td>
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
                  <td className="p-3 text-muted-foreground text-xs">
                    {formatTime(user.created_at)}
                  </td>
                  <td className="p-3 text-right">
                    {deleteId === user.id ? (
                      <span className="inline-flex items-center gap-2">
                        <span className="text-xs text-muted-foreground">
                          Delete?
                        </span>
                        <Button
                          variant="destructive"
                          size="sm"
                          disabled={acting === user.id}
                          onClick={() => handleDelete(user.id)}
                        >
                          {acting === user.id ? "Deleting..." : "Confirm"}
                        </Button>
                        <Button
                          variant="ghost"
                          size="sm"
                          onClick={() => setDeleteId(null)}
                        >
                          Cancel
                        </Button>
                      </span>
                    ) : (
                      <span className="inline-flex items-center gap-1">
                        <Button
                          variant="ghost"
                          size="sm"
                          title="Impersonate"
                          disabled={!user.is_active}
                          onClick={() => handleImpersonate(user.id)}
                        >
                          <UserCheck className="h-3.5 w-3.5 text-amber-600" />
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
                          onClick={() => setDeleteId(user.id)}
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
      {totalPages > 1 && (
        <div className="flex items-center justify-between mt-4">
          <p className="text-sm text-muted-foreground">
            Showing {page * pageSize + 1}&ndash;{Math.min((page + 1) * pageSize, total)} of {total}
          </p>
          <div className="flex items-center gap-2">
            <Button
              variant="outline"
              size="sm"
              disabled={page === 0}
              onClick={() => setPage(page - 1)}
            >
              <ChevronLeft className="h-4 w-4" />
            </Button>
            <span className="text-sm text-muted-foreground">
              {page + 1} / {totalPages}
            </span>
            <Button
              variant="outline"
              size="sm"
              disabled={page >= totalPages - 1}
              onClick={() => setPage(page + 1)}
            >
              <ChevronRight className="h-4 w-4" />
            </Button>
          </div>
        </div>
      )}

      {/* Create / Edit Dialog */}
      <Dialog open={dialogOpen} onClose={closeDialog}>
        <DialogHeader>
          <DialogTitle>{editing ? "Edit User" : "Create User"}</DialogTitle>
          <DialogCloseButton onClick={closeDialog} />
        </DialogHeader>

        <form onSubmit={handleSubmit}>
          <DialogBody className="space-y-4">
            {formError && (
              <div className="p-3 bg-red-50 border border-red-200 text-red-700 rounded-md text-sm">
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
              />
            </div>

            <div className="space-y-2">
              <Label htmlFor="name">Name</Label>
              <Input
                id="name"
                value={name}
                onChange={(e) => setName(e.target.value)}
                placeholder="Full name"
              />
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
              />
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
                <Check className="h-3.5 w-3.5 text-green-600" />
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
    </div>
  );
}

function StatusBadge({ active }: { active: boolean }) {
  return (
    <span
      className={`inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium ${
        active ? "bg-green-100 text-green-700" : "bg-red-100 text-red-700"
      }`}
    >
      {active ? "Active" : "Inactive"}
    </span>
  );
}
