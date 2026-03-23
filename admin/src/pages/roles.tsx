import { useCallback, useEffect, useState } from "react";
import {
  ActionIcon,
  Alert,
  Badge,
  Button,
  Checkbox,
  Group,
  Loader,
  Modal,
  Stack,
  Table,
  Text,
  TextInput,
  Textarea,
  Title,
  Tooltip,
} from "@mantine/core";
import { useForm } from "@mantine/form";
import { notifications } from "@mantine/notifications";
import {
  IconAlertCircle,
  IconCheck,
  IconPencil,
  IconPlus,
  IconTrash,
} from "@tabler/icons-react";
import { del, get, post, put, ApiError } from "@/lib/api";

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

interface ScopeDef {
  name: string;
  label: string;
}

export default function RolesPage() {
  const [roles, setRoles] = useState<Role[]>([]);
  const [allScopes, setAllScopes] = useState<ScopeDef[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  const [createOpen, setCreateOpen] = useState(false);
  const [editRole, setEditRole] = useState<Role | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<Role | null>(null);
  const [actionLoading, setActionLoading] = useState(false);

  const createForm = useForm({
    initialValues: { name: "", description: "", scopes: [] as string[] },
    validate: {
      name: (v) => (!v ? "Name is required" : v.length < 2 ? "Minimum 2 characters" : null),
    },
  });

  const editForm = useForm({
    initialValues: { name: "", description: "", scopes: [] as string[] },
  });

  const load = useCallback(async () => {
    setLoading(true);
    setError("");
    try {
      const [rolesData, scopesData] = await Promise.all([
        get<{ roles: Role[] }>("/admin/roles/"),
        get<{ scopes: ScopeDef[] }>("/admin/roles/scopes"),
      ]);
      setRoles(rolesData.roles ?? []);
      setAllScopes(scopesData.scopes ?? []);
    } catch {
      setError("Failed to load roles");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => { load(); }, [load]);

  async function handleCreate(values: typeof createForm.values) {
    setActionLoading(true);
    try {
      await post("/admin/roles/", values);
      notifications.show({ message: "Role created", color: "green", icon: <IconCheck size={16} /> });
      setCreateOpen(false);
      createForm.reset();
      load();
    } catch (e) {
      if (e instanceof ApiError && Object.keys(e.fields).length > 0) {
        createForm.setErrors(e.fields);
      } else {
        notifications.show({ message: e instanceof ApiError ? e.message : "Failed to create role", color: "red" });
      }
    } finally {
      setActionLoading(false);
    }
  }

  function openEdit(role: Role) {
    setEditRole(role);
    editForm.setValues({ name: role.name, description: role.description, scopes: [...role.scopes] });
  }

  async function handleEdit(values: typeof editForm.values) {
    if (!editRole) return;
    setActionLoading(true);
    try {
      await put(`/admin/roles/${editRole.id}`, values);
      notifications.show({ message: "Role updated", color: "green", icon: <IconCheck size={16} /> });
      setEditRole(null);
      load();
    } catch (e) {
      if (e instanceof ApiError && Object.keys(e.fields).length > 0) {
        editForm.setErrors(e.fields);
      } else {
        notifications.show({ message: e instanceof ApiError ? e.message : "Failed to update role", color: "red" });
      }
    } finally {
      setActionLoading(false);
    }
  }

  async function handleDelete() {
    if (!deleteTarget) return;
    setActionLoading(true);
    try {
      await del(`/admin/roles/${deleteTarget.id}`);
      notifications.show({ message: "Role deleted", color: "green", icon: <IconCheck size={16} /> });
      setDeleteTarget(null);
      load();
    } catch (e) {
      notifications.show({ message: e instanceof ApiError ? e.message : "Failed to delete role", color: "red" });
    } finally {
      setActionLoading(false);
    }
  }

  function ScopeCheckboxes({ form, field }: { form: ReturnType<typeof useForm<{ name: string; description: string; scopes: string[] }>>; field: string }) {
    const values = form.getValues().scopes;
    return (
      <Stack gap="xs">
        <Text size="sm" fw={500}>Scopes</Text>
        {allScopes.map((scope) => (
          <Checkbox
            key={scope.name}
            label={scope.label}
            checked={values.includes(scope.name)}
            disabled={scope.name === "admin"}
            onChange={(e) => {
              const checked = e.currentTarget.checked;
              const current = form.getValues().scopes;
              form.setFieldValue(
                field as "scopes",
                checked ? [...current, scope.name] : current.filter((s) => s !== scope.name),
              );
            }}
          />
        ))}
      </Stack>
    );
  }

  return (
    <Stack>
      <Group justify="space-between" wrap="wrap">
        <Group gap="xs">
          <Title order={3}>Roles</Title>
          {!loading && <Badge variant="light" size="lg">{roles.length}</Badge>}
        </Group>
        <Button leftSection={<IconPlus size={16} />} onClick={() => { createForm.reset(); createForm.setFieldValue("scopes", ["admin"]); setCreateOpen(true); }}>
          Create Role
        </Button>
      </Group>

      {error && (
        <Alert icon={<IconAlertCircle size={16} />} color="red" variant="light" withCloseButton onClose={() => setError("")}>
          {error}
          <Button variant="subtle" size="xs" ml="sm" onClick={load}>Retry</Button>
        </Alert>
      )}

      {loading ? (
        <Group justify="center" pt="xl"><Loader /></Group>
      ) : (
        <Table.ScrollContainer minWidth={700}>
          <Table>
            <Table.Thead>
              <Table.Tr>
                <Table.Th>Name</Table.Th>
                <Table.Th>Description</Table.Th>
                <Table.Th>Scopes</Table.Th>
                <Table.Th>Admins</Table.Th>
                <Table.Th ta="right">Actions</Table.Th>
              </Table.Tr>
            </Table.Thead>
            <Table.Tbody>
              {roles.length === 0 ? (
                <Table.Tr>
                  <Table.Td colSpan={5}>
                    <Text ta="center" c="dimmed" py="lg">No roles yet</Text>
                  </Table.Td>
                </Table.Tr>
              ) : (
                roles.map((role) => (
                  <Table.Tr key={role.id}>
                    <Table.Td>
                      <Group gap={6}>
                        <Text size="sm" fw={500}>{role.name}</Text>
                        {role.is_system && <Badge variant="light" color="gray" size="xs">system</Badge>}
                      </Group>
                    </Table.Td>
                    <Table.Td>
                      <Text size="sm" c="dimmed">{role.description || "—"}</Text>
                    </Table.Td>
                    <Table.Td>
                      <Group gap={4} wrap="wrap">
                        {role.scopes.map((scope) => {
                          const def = allScopes.find((s) => s.name === scope);
                          return (
                            <Tooltip key={scope} label={scope}>
                              <Badge variant="light" size="xs">{def?.label ?? scope}</Badge>
                            </Tooltip>
                          );
                        })}
                      </Group>
                    </Table.Td>
                    <Table.Td>
                      <Text size="sm">{role.admin_count}</Text>
                    </Table.Td>
                    <Table.Td>
                      <Group gap={4} justify="flex-end" wrap="nowrap">
                        <Tooltip label="Edit">
                          <ActionIcon variant="subtle" size="sm" onClick={() => openEdit(role)}>
                            <IconPencil size={16} />
                          </ActionIcon>
                        </Tooltip>
                        {!role.is_system && (
                          <Tooltip label="Delete">
                            <ActionIcon variant="subtle" size="sm" color="red" onClick={() => setDeleteTarget(role)}>
                              <IconTrash size={16} />
                            </ActionIcon>
                          </Tooltip>
                        )}
                      </Group>
                    </Table.Td>
                  </Table.Tr>
                ))
              )}
            </Table.Tbody>
          </Table>
        </Table.ScrollContainer>
      )}

      {/* Create modal */}
      <Modal opened={createOpen} onClose={() => setCreateOpen(false)} title="Create Role">
        <form onSubmit={createForm.onSubmit(handleCreate)}>
          <Stack>
            <TextInput label="Name" placeholder="e.g. editor" required {...createForm.getInputProps("name")} />
            <Textarea label="Description" placeholder="What this role is for" {...createForm.getInputProps("description")} />
            <ScopeCheckboxes form={createForm} field="scopes" />
            <Group justify="flex-end">
              <Button variant="default" onClick={() => setCreateOpen(false)}>Cancel</Button>
              <Button type="submit" loading={actionLoading}>Create</Button>
            </Group>
          </Stack>
        </form>
      </Modal>

      {/* Edit modal */}
      <Modal opened={!!editRole} onClose={() => setEditRole(null)} title="Edit Role">
        <form onSubmit={editForm.onSubmit(handleEdit)}>
          <Stack>
            <TextInput label="Name" disabled={editRole?.is_system} {...editForm.getInputProps("name")} />
            <Textarea label="Description" {...editForm.getInputProps("description")} />
            <ScopeCheckboxes form={editForm} field="scopes" />
            <Group justify="flex-end">
              <Button variant="default" onClick={() => setEditRole(null)}>Cancel</Button>
              <Button type="submit" loading={actionLoading}>Save</Button>
            </Group>
          </Stack>
        </form>
      </Modal>

      {/* Delete confirmation */}
      <Modal opened={!!deleteTarget} onClose={() => setDeleteTarget(null)} title="Delete Role" size="sm">
        <Stack>
          <Text size="sm">
            Delete role <strong>{deleteTarget?.name}</strong>?
            {deleteTarget && deleteTarget.admin_count > 0 && (
              <Text c="red" size="sm" mt={4}>This role is assigned to {deleteTarget.admin_count} admin(s).</Text>
            )}
          </Text>
          <Group justify="flex-end">
            <Button variant="default" onClick={() => setDeleteTarget(null)}>Cancel</Button>
            <Button color="red" onClick={handleDelete} loading={actionLoading}>Delete</Button>
          </Group>
        </Stack>
      </Modal>
    </Stack>
  );
}
