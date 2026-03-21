import { useCallback, useEffect, useState } from "react";
import { toast } from "sonner";
import { get, post, put, del, ApiError } from "@/lib/api";
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
import { Plus, Pencil, Trash2 } from "lucide-react";
import { TableSkeleton } from "@/components/ui/skeleton";
import { ErrorAlert } from "@/components/ui/error-alert";
import { TableEmptyRow } from "@/components/ui/empty-state";

interface Role {
  id: number;
  name: string;
  description: string;
  is_system: boolean;
  scopes: string[];
  admin_count: number;
  created_at: string;
  updated_at: string;
}

const SCOPE_LABELS: Record<string, string> = {
  admin: "Base Access",
  "admin:users": "User Management",
  "admin:settings": "Settings",
  "admin:jobs": "Jobs & Cron",
  "admin:logs": "Log Viewer",
  "admin:audit": "Audit Log",
  "admin:uploads": "Uploads",
  "admin:database": "Database",
  "admin:roles": "Role Management",
};

export default function RolesPage() {
  const [roles, setRoles] = useState<Role[]>([]);
  const [knownScopes, setKnownScopes] = useState<string[]>([]);
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(true);
  const [acting, setActing] = useState<number | null>(null);

  const [dialogOpen, setDialogOpen] = useState(false);
  const [editing, setEditing] = useState<Role | null>(null);
  const [deleteId, setDeleteId] = useState<number | null>(null);

  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [selectedScopes, setSelectedScopes] = useState<string[]>([]);
  const [formError, setFormError] = useState("");
  const [submitting, setSubmitting] = useState(false);

  const load = useCallback(async () => {
    try {
      const [rolesData, scopesData] = await Promise.all([
        get<{ roles: Role[] }>("/admin/roles/"),
        get<{ scopes: string[] }>("/admin/roles/scopes"),
      ]);
      setRoles(rolesData.roles);
      setKnownScopes(scopesData.scopes);
      setError("");
    } catch (e: any) {
      setError(e.message || "Failed to load roles");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    load();
  }, [load]);

  function openCreate() {
    setEditing(null);
    setName("");
    setDescription("");
    setSelectedScopes(["admin"]);
    setFormError("");
    setDialogOpen(true);
  }

  function openEdit(role: Role) {
    setEditing(role);
    setName(role.name);
    setDescription(role.description);
    setSelectedScopes([...role.scopes]);
    setFormError("");
    setDialogOpen(true);
  }

  function closeDialog() {
    setDialogOpen(false);
    setEditing(null);
  }

  function toggleScope(scope: string) {
    if (scope === "admin") return; // Base scope always required.
    setSelectedScopes((prev) =>
      prev.includes(scope)
        ? prev.filter((s) => s !== scope)
        : [...prev, scope]
    );
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setFormError("");
    setSubmitting(true);

    try {
      if (editing) {
        const body: Record<string, unknown> = {
          description,
          scopes: selectedScopes,
        };
        if (!editing.is_system) body.name = name;
        await put(`/admin/roles/${editing.id}`, body);
        toast.success("Role updated");
      } else {
        await post("/admin/roles/", { name, description, scopes: selectedScopes });
        toast.success("Role created");
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

  async function handleDelete(id: number) {
    setActing(id);
    try {
      await del(`/admin/roles/${id}`);
      setDeleteId(null);
      toast.success("Role deleted");
      await load();
    } catch (e: any) {
      toast.error(e.message || "Failed to delete role");
    } finally {
      setActing(null);
    }
  }

  if (loading) {
    return (
      <div className="p-6">
        <h1 className="text-2xl font-bold mb-6">Roles & Permissions</h1>
        <TableSkeleton columns={[
          { width: "w-24" },
          { width: "w-32", hidden: "hidden md:table-cell" },
          { width: "w-40" },
          { width: "w-14", hidden: "hidden md:table-cell" },
          { width: "w-20" },
        ]} rows={3} />
      </div>
    );
  }

  return (
    <div className="p-6">
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-2xl font-bold">Roles & Permissions</h1>
          <p className="text-sm text-muted-foreground mt-1">
            Manage admin roles and their associated scopes
          </p>
        </div>
        <Button onClick={openCreate}>
          <Plus className="h-4 w-4 mr-2" />
          Create Role
        </Button>
      </div>

      {error && (
        <ErrorAlert
          message={error}
          onRetry={load}
          onDismiss={() => setError("")}
          className="mb-4"
        />
      )}

      <div className="border rounded-lg overflow-hidden">
        <table className="w-full text-sm">
          <thead>
            <tr className="bg-muted/50 border-b">
              <th className="text-left p-3 font-medium">Role</th>
              <th className="text-left p-3 font-medium hidden md:table-cell">
                Description
              </th>
              <th className="text-left p-3 font-medium">Scopes</th>
              <th className="text-left p-3 font-medium hidden md:table-cell">
                Admins
              </th>
              <th className="text-right p-3 font-medium">Actions</th>
            </tr>
          </thead>
          <tbody>
            {roles.length === 0 ? (
              <TableEmptyRow colSpan={5} message="No roles found" />
            ) : (
              roles.map((role) => (
                <tr
                  key={role.id}
                  className="border-b last:border-0 hover:bg-muted/30"
                >
                  <td className="p-3">
                    <div className="flex items-center gap-2">
                      <span className="font-medium">{role.name}</span>
                      {role.is_system && (
                        <span className="inline-flex items-center px-1.5 py-0.5 rounded text-[10px] font-medium bg-amber-100 text-amber-700">
                          system
                        </span>
                      )}
                    </div>
                  </td>
                  <td className="p-3 text-muted-foreground hidden md:table-cell">
                    {role.description || "\u2014"}
                  </td>
                  <td className="p-3">
                    <div className="flex flex-wrap gap-1">
                      {role.scopes.map((scope) => (
                        <ScopeBadge key={scope} scope={scope} />
                      ))}
                    </div>
                  </td>
                  <td className="p-3 hidden md:table-cell">
                    <span className="text-muted-foreground">
                      {role.admin_count}
                    </span>
                  </td>
                  <td className="p-3 text-right">
                    {deleteId === role.id ? (
                      <span className="inline-flex items-center gap-2">
                        <span className="text-xs text-muted-foreground">
                          Delete?
                        </span>
                        <Button
                          variant="destructive"
                          size="sm"
                          disabled={acting === role.id}
                          onClick={() => handleDelete(role.id)}
                        >
                          {acting === role.id ? "Deleting..." : "Confirm"}
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
                          onClick={() => openEdit(role)}
                        >
                          <Pencil className="h-3.5 w-3.5" />
                        </Button>
                        {!role.is_system && (
                          <Button
                            variant="ghost"
                            size="sm"
                            onClick={() => setDeleteId(role.id)}
                          >
                            <Trash2 className="h-3.5 w-3.5 text-red-500" />
                          </Button>
                        )}
                      </span>
                    )}
                  </td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>

      {/* Create / Edit Dialog */}
      <Dialog open={dialogOpen} onClose={closeDialog}>
        <DialogHeader>
          <DialogTitle>
            {editing ? "Edit Role" : "Create Role"}
          </DialogTitle>
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
              <Label htmlFor="name">Name</Label>
              <Input
                id="name"
                value={name}
                onChange={(e) => setName(e.target.value)}
                disabled={editing?.is_system}
                placeholder="e.g. editor, moderator"
              />
              {editing?.is_system && (
                <p className="text-xs text-muted-foreground">
                  System role names cannot be changed
                </p>
              )}
            </div>

            <div className="space-y-2">
              <Label htmlFor="description">Description</Label>
              <Input
                id="description"
                value={description}
                onChange={(e) => setDescription(e.target.value)}
                placeholder="Brief description of this role"
              />
            </div>

            <div className="space-y-2">
              <Label>Permissions</Label>
              <div className="border rounded-md divide-y">
                {knownScopes.map((scope) => (
                  <label
                    key={scope}
                    className="flex items-center gap-3 px-3 py-2.5 hover:bg-muted/30 cursor-pointer"
                  >
                    <input
                      type="checkbox"
                      checked={selectedScopes.includes(scope)}
                      onChange={() => toggleScope(scope)}
                      disabled={scope === "admin"}
                      className="h-4 w-4 rounded border-gray-300"
                    />
                    <div className="flex-1 min-w-0">
                      <div className="text-sm font-medium">
                        {SCOPE_LABELS[scope] || scope}
                      </div>
                      <div className="text-xs text-muted-foreground font-mono">
                        {scope}
                      </div>
                    </div>
                  </label>
                ))}
              </div>
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
                  : "Create Role"}
            </Button>
          </DialogFooter>
        </form>
      </Dialog>
    </div>
  );
}

function ScopeBadge({ scope }: { scope: string }) {
  const colors: Record<string, string> = {
    admin: "bg-gray-100 text-gray-700",
    "admin:users": "bg-blue-100 text-blue-700",
    "admin:settings": "bg-green-100 text-green-700",
    "admin:jobs": "bg-orange-100 text-orange-700",
    "admin:logs": "bg-purple-100 text-purple-700",
    "admin:audit": "bg-pink-100 text-pink-700",
    "admin:uploads": "bg-cyan-100 text-cyan-700",
    "admin:database": "bg-red-100 text-red-700",
    "admin:roles": "bg-amber-100 text-amber-700",
  };
  return (
    <span
      className={`inline-flex items-center px-1.5 py-0.5 rounded text-[10px] font-medium ${colors[scope] || "bg-gray-100 text-gray-700"}`}
    >
      {SCOPE_LABELS[scope] || scope}
    </span>
  );
}
