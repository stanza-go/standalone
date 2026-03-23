import { useCallback, useEffect, useState } from "react";
import { useNavigate } from "react-router";
import {
  ActionIcon,
  Alert,
  Badge,
  Box,
  Button,
  Checkbox,
  Group,
  Loader,
  Modal,
  Pagination,
  Select,
  Stack,
  Table,
  Text,
  TextInput,
  Title,
  Tooltip,
} from "@mantine/core";
import { useForm } from "@mantine/form";
import { notifications } from "@mantine/notifications";
import {
  IconAlertCircle,
  IconArrowDown,
  IconArrowUp,
  IconArrowsSort,
  IconCheck,
  IconDownload,
  IconPencil,
  IconPlus,
  IconSearch,
  IconTrash,
  IconShield,
  IconX,
} from "@tabler/icons-react";
import { del, downloadCSV, get, post, put, ApiError } from "@/lib/api";
import { EmptyState } from "@/components/empty-state";
import { useAuth } from "@/lib/auth";
import { useDebounce } from "@/hooks/use-debounce";
import { useSort } from "@/hooks/use-sort";
import { useSelection } from "@/hooks/use-selection";
import { useTableKeyboard } from "@/hooks/use-table-keyboard";

interface Admin {
  id: number;
  email: string;
  name: string;
  role: string;
  is_active: boolean;
  created_at: string;
  updated_at: string;
}

const PAGE_SIZE = 20;

function SortIcon({ column, sort }: { column: string; sort: { column: string; direction: string } }) {
  if (sort.column !== column) return <IconArrowsSort size={14} stroke={1.5} />;
  return sort.direction === "asc" ? <IconArrowUp size={14} stroke={1.5} /> : <IconArrowDown size={14} stroke={1.5} />;
}

function timeAgo(dateStr: string): string {
  const seconds = Math.floor((Date.now() - new Date(dateStr).getTime()) / 1000);
  if (seconds < 60) return "just now";
  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) return `${minutes}m ago`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours}h ago`;
  const days = Math.floor(hours / 24);
  if (days < 30) return `${days}d ago`;
  return new Date(dateStr).toLocaleDateString();
}

export default function AdminsPage() {
  const navigate = useNavigate();
  const { admin: currentAdmin } = useAuth();
  const [admins, setAdmins] = useState<Admin[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [searchInput, setSearchInput] = useState("");
  const search = useDebounce(searchInput, 300);
  const [sort, toggleSort] = useSort("id", "asc");
  const selection = useSelection();
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [roleNames, setRoleNames] = useState<string[]>([]);

  // Modal states
  const [createOpen, setCreateOpen] = useState(false);
  const [editAdmin, setEditAdmin] = useState<Admin | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<Admin | null>(null);
  const [bulkDeleteOpen, setBulkDeleteOpen] = useState(false);
  const [actionLoading, setActionLoading] = useState(false);

  const createForm = useForm({
    initialValues: { email: "", password: "", name: "", role: "admin" },
    validate: {
      email: (v) => (!v ? "Email is required" : null),
      password: (v) => (!v ? "Password is required" : v.length < 8 ? "Minimum 8 characters" : null),
    },
  });

  const editForm = useForm({
    initialValues: { name: "", role: "admin", password: "" },
  });

  const load = useCallback(async () => {
    setLoading(true);
    setError("");
    try {
      const offset = (page - 1) * PAGE_SIZE;
      const params = new URLSearchParams({
        limit: String(PAGE_SIZE),
        offset: String(offset),
        sort: sort.column,
        order: sort.direction.toUpperCase(),
      });
      if (search) params.set("search", search);
      const data = await get<{ admins: Admin[]; total: number }>(`/admin/admins?${params}`);
      setAdmins(data.admins ?? []);
      setTotal(data.total);
    } catch {
      setError("Failed to load admins");
    } finally {
      setLoading(false);
    }
  }, [page, search, sort]);

  useEffect(() => {
    load();
  }, [load]);

  useEffect(() => {
    get<{ roles: string[] }>("/admin/role-names").then((d) => setRoleNames(d.roles)).catch(() => {});
  }, []);

  useEffect(() => {
    setPage(1);
    selection.clear();
  }, [search]); // eslint-disable-line react-hooks/exhaustive-deps

  useEffect(() => {
    selection.clear();
  }, [page, sort]); // eslint-disable-line react-hooks/exhaustive-deps

  const totalPages = Math.ceil(total / PAGE_SIZE);

  async function handleCreate(values: typeof createForm.values) {
    setActionLoading(true);
    try {
      await post("/admin/admins", values);
      notifications.show({ message: "Admin created", color: "green", icon: <IconCheck size={16} /> });
      setCreateOpen(false);
      createForm.reset();
      load();
    } catch (e) {
      if (e instanceof ApiError && Object.keys(e.fields).length > 0) {
        createForm.setErrors(e.fields);
      } else {
        notifications.show({ message: e instanceof ApiError ? e.message : "Failed to create admin", color: "red" });
      }
    } finally {
      setActionLoading(false);
    }
  }

  function openEdit(admin: Admin) {
    setEditAdmin(admin);
    editForm.setValues({ name: admin.name, role: admin.role, password: "" });
  }

  async function handleEdit(values: typeof editForm.values) {
    if (!editAdmin) return;
    setActionLoading(true);
    try {
      const body: Record<string, unknown> = { name: values.name, role: values.role };
      if (values.password) body.password = values.password;
      await put(`/admin/admins/${editAdmin.id}`, body);
      notifications.show({ message: "Admin updated", color: "green", icon: <IconCheck size={16} /> });
      setEditAdmin(null);
      load();
    } catch (e) {
      if (e instanceof ApiError && Object.keys(e.fields).length > 0) {
        editForm.setErrors(e.fields);
      } else {
        notifications.show({ message: e instanceof ApiError ? e.message : "Failed to update admin", color: "red" });
      }
    } finally {
      setActionLoading(false);
    }
  }

  async function handleToggleActive(admin: Admin) {
    if (admin.id === currentAdmin?.id) {
      notifications.show({ message: "Cannot deactivate your own account", color: "yellow" });
      return;
    }
    try {
      await put(`/admin/admins/${admin.id}`, { is_active: !admin.is_active });
      notifications.show({
        message: `Admin ${admin.is_active ? "deactivated" : "activated"}`,
        color: "green",
        icon: <IconCheck size={16} />,
      });
      load();
    } catch {
      notifications.show({ message: "Failed to update status", color: "red" });
    }
  }

  async function handleDelete() {
    if (!deleteTarget) return;
    setActionLoading(true);
    try {
      await del(`/admin/admins/${deleteTarget.id}`);
      notifications.show({ message: "Admin deleted", color: "green", icon: <IconCheck size={16} /> });
      setDeleteTarget(null);
      load();
    } catch (e) {
      notifications.show({ message: e instanceof ApiError ? e.message : "Failed to delete admin", color: "red" });
    } finally {
      setActionLoading(false);
    }
  }

  async function handleBulkDelete() {
    setActionLoading(true);
    try {
      const data = await post<{ affected: number }>("/admin/admins/bulk-delete", { ids: selection.ids });
      notifications.show({ message: `${data.affected} admin(s) deleted`, color: "green", icon: <IconCheck size={16} /> });
      setBulkDeleteOpen(false);
      selection.clear();
      load();
    } catch (e) {
      notifications.show({ message: e instanceof ApiError ? e.message : "Failed to delete admins", color: "red" });
    } finally {
      setActionLoading(false);
    }
  }

  async function handleExport() {
    try {
      const params = new URLSearchParams({ sort: sort.column, order: sort.direction.toUpperCase() });
      if (search) params.set("search", search);
      await downloadCSV(`/admin/admins/export?${params}`);
    } catch {
      notifications.show({ message: "Failed to export", color: "red" });
    }
  }

  const adminIds = admins.map((a) => a.id);

  const tableKeyboard = useTableKeyboard({
    rowCount: admins.length,
    onActivate: (i) => { const a = admins[i]; if (a) navigate(`/admins/${a.id}`); },
    onSelect: (i) => { const a = admins[i]; if (a) selection.toggle(a.id); },
  });

  return (
    <Stack>
      <Group justify="space-between" wrap="wrap">
        <Group gap="xs">
          <Title order={3}>Admin Users</Title>
          {!loading && <Badge variant="light" size="lg">{total}</Badge>}
        </Group>
        <Group gap="xs">
          <Button variant="subtle" size="xs" leftSection={<IconDownload size={16} />} onClick={handleExport}>
            Export CSV
          </Button>
          <Button leftSection={<IconPlus size={16} />} onClick={() => setCreateOpen(true)}>
            Create Admin
          </Button>
        </Group>
      </Group>

      <TextInput
        placeholder="Search by email or name..."
        leftSection={<IconSearch size={16} />}
        value={searchInput}
        onChange={(e) => setSearchInput(e.currentTarget.value)}
        rightSection={
          searchInput ? (
            <ActionIcon variant="subtle" size="sm" onClick={() => setSearchInput("")}>
              <IconX size={14} />
            </ActionIcon>
          ) : null
        }
      />

      {error && (
        <Alert icon={<IconAlertCircle size={16} />} color="red" variant="light" withCloseButton onClose={() => setError("")}>
          {error}
          <Button variant="subtle" size="xs" ml="sm" onClick={load}>Retry</Button>
        </Alert>
      )}

      {loading && admins.length === 0 ? (
        <Group justify="center" pt="xl"><Loader /></Group>
      ) : admins.length === 0 ? (
        <EmptyState
          icon={<IconShield size={24} />}
          title={search ? "No admins match your search" : "No admin users yet"}
          description={search ? "Try a different search term." : "Create an admin to manage your application."}
          action={!search ? <Button leftSection={<IconPlus size={16} />} onClick={() => setCreateOpen(true)}>Create Admin</Button> : undefined}
        />
      ) : (
        <>
          <Table.ScrollContainer minWidth={700}>
            <Table>
              <Table.Thead>
                <Table.Tr>
                  <Table.Th w={40}>
                    <Checkbox
                      checked={selection.isAllSelected(adminIds)}
                      indeterminate={selection.count > 0 && !selection.isAllSelected(adminIds)}
                      onChange={() => selection.toggleAll(adminIds)}
                      aria-label="Select all"
                    />
                  </Table.Th>
                  <Table.Th w={60} style={{ cursor: "pointer" }} onClick={() => toggleSort("id")}>
                    <Group gap={4} wrap="nowrap">ID <SortIcon column="id" sort={sort} /></Group>
                  </Table.Th>
                  <Table.Th style={{ cursor: "pointer" }} onClick={() => toggleSort("email")}>
                    <Group gap={4} wrap="nowrap">Email <SortIcon column="email" sort={sort} /></Group>
                  </Table.Th>
                  <Table.Th style={{ cursor: "pointer" }} onClick={() => toggleSort("name")}>
                    <Group gap={4} wrap="nowrap">Name <SortIcon column="name" sort={sort} /></Group>
                  </Table.Th>
                  <Table.Th style={{ cursor: "pointer" }} onClick={() => toggleSort("role")}>
                    <Group gap={4} wrap="nowrap">Role <SortIcon column="role" sort={sort} /></Group>
                  </Table.Th>
                  <Table.Th style={{ cursor: "pointer" }} onClick={() => toggleSort("is_active")}>
                    <Group gap={4} wrap="nowrap">Status <SortIcon column="is_active" sort={sort} /></Group>
                  </Table.Th>
                  <Table.Th style={{ cursor: "pointer" }} onClick={() => toggleSort("created_at")}>
                    <Group gap={4} wrap="nowrap">Created <SortIcon column="created_at" sort={sort} /></Group>
                  </Table.Th>
                  <Table.Th ta="right">Actions</Table.Th>
                </Table.Tr>
              </Table.Thead>
              <Table.Tbody {...tableKeyboard.tbodyProps}>
                {admins.map((admin, idx) => (
                    <Table.Tr
                      key={admin.id}
                      bg={selection.isSelected(admin.id) ? "var(--mantine-primary-color-light)" : undefined}
                      style={tableKeyboard.isFocused(idx) ? { outline: "2px solid var(--mantine-primary-color-filled)", outlineOffset: -2 } : undefined}
                    >
                      <Table.Td>
                        <Checkbox
                          checked={selection.isSelected(admin.id)}
                          onChange={() => selection.toggle(admin.id)}
                          aria-label={`Select ${admin.email}`}
                        />
                      </Table.Td>
                      <Table.Td>
                        <Text size="sm" c="dimmed">{admin.id}</Text>
                      </Table.Td>
                      <Table.Td>
                        <Text
                          size="sm"
                          fw={500}
                          style={{ cursor: "pointer" }}
                          c="blue"
                          onClick={() => navigate(`/admins/${admin.id}`)}
                        >
                          {admin.email}
                          {admin.id === currentAdmin?.id && (
                            <Badge variant="light" size="xs" ml={6}>you</Badge>
                          )}
                        </Text>
                      </Table.Td>
                      <Table.Td>
                        <Text size="sm">{admin.name || "—"}</Text>
                      </Table.Td>
                      <Table.Td>
                        <Badge variant="light" color="violet" size="sm">{admin.role}</Badge>
                      </Table.Td>
                      <Table.Td>
                        <Badge
                          variant="light"
                          color={admin.is_active ? "green" : "red"}
                          leftSection={admin.is_active ? <IconCheck size={10} /> : <IconX size={10} />}
                          style={{ cursor: admin.id !== currentAdmin?.id ? "pointer" : undefined }}
                          onClick={() => handleToggleActive(admin)}
                        >
                          {admin.is_active ? "Active" : "Inactive"}
                        </Badge>
                      </Table.Td>
                      <Table.Td>
                        <Tooltip label={new Date(admin.created_at).toLocaleString()}>
                          <Text size="sm" c="dimmed">{timeAgo(admin.created_at)}</Text>
                        </Tooltip>
                      </Table.Td>
                      <Table.Td>
                        <Group gap={4} justify="flex-end" wrap="nowrap">
                          <Tooltip label="Edit">
                            <ActionIcon variant="subtle" size="sm" onClick={() => openEdit(admin)}>
                              <IconPencil size={16} />
                            </ActionIcon>
                          </Tooltip>
                          {admin.id !== currentAdmin?.id && (
                            <Tooltip label="Delete">
                              <ActionIcon variant="subtle" size="sm" color="red" onClick={() => setDeleteTarget(admin)}>
                                <IconTrash size={16} />
                              </ActionIcon>
                            </Tooltip>
                          )}
                        </Group>
                      </Table.Td>
                    </Table.Tr>
                  ))}
              </Table.Tbody>
            </Table>
          </Table.ScrollContainer>

          {totalPages > 1 && (
            <Group justify="space-between">
              <Text size="sm" c="dimmed">
                Showing {(page - 1) * PAGE_SIZE + 1}–{Math.min(page * PAGE_SIZE, total)} of {total}
              </Text>
              <Pagination value={page} onChange={setPage} total={totalPages} size="sm" />
            </Group>
          )}
        </>
      )}

      {selection.count > 0 && (
        <Box pos="fixed" bottom={20} left="50%" style={{ transform: "translateX(-50%)", zIndex: 100 }}>
          <Group
            gap="sm" px="md" py="xs"
            style={(theme) => ({
              background: "var(--mantine-color-body)",
              border: "1px solid var(--mantine-color-default-border)",
              borderRadius: theme.radius.md,
              boxShadow: theme.shadows.lg,
            })}
          >
            <Text size="sm" fw={500}>{selection.count} selected</Text>
            <Button variant="light" color="red" size="xs" leftSection={<IconTrash size={14} />} onClick={() => setBulkDeleteOpen(true)}>
              Delete
            </Button>
            <ActionIcon variant="subtle" size="sm" onClick={selection.clear}><IconX size={14} /></ActionIcon>
          </Group>
        </Box>
      )}

      {/* Create modal */}
      <Modal opened={createOpen} onClose={() => setCreateOpen(false)} title="Create Admin">
        <form onSubmit={createForm.onSubmit(handleCreate)}>
          <Stack>
            <TextInput label="Email" placeholder="admin@example.com" required {...createForm.getInputProps("email")} />
            <TextInput label="Password" type="password" placeholder="Minimum 8 characters" required {...createForm.getInputProps("password")} />
            <TextInput label="Name" placeholder="Full name" {...createForm.getInputProps("name")} />
            <Select label="Role" data={roleNames} {...createForm.getInputProps("role")} />
            <Group justify="flex-end">
              <Button variant="default" onClick={() => setCreateOpen(false)}>Cancel</Button>
              <Button type="submit" loading={actionLoading}>Create</Button>
            </Group>
          </Stack>
        </form>
      </Modal>

      {/* Edit modal */}
      <Modal opened={!!editAdmin} onClose={() => setEditAdmin(null)} title="Edit Admin">
        <form onSubmit={editForm.onSubmit(handleEdit)}>
          <Stack>
            <TextInput label="Email" value={editAdmin?.email ?? ""} disabled />
            <TextInput label="Name" placeholder="Full name" {...editForm.getInputProps("name")} />
            <Select label="Role" data={roleNames} {...editForm.getInputProps("role")} />
            <TextInput label="Password" type="password" placeholder="Leave empty to keep current" {...editForm.getInputProps("password")} />
            <Group justify="flex-end">
              <Button variant="default" onClick={() => setEditAdmin(null)}>Cancel</Button>
              <Button type="submit" loading={actionLoading}>Save</Button>
            </Group>
          </Stack>
        </form>
      </Modal>

      {/* Delete confirmation */}
      <Modal opened={!!deleteTarget} onClose={() => setDeleteTarget(null)} title="Delete Admin">
        <Stack>
          <Text size="sm">
            Are you sure you want to delete <strong>{deleteTarget?.email}</strong>? This action cannot be undone.
          </Text>
          <Group justify="flex-end">
            <Button variant="default" onClick={() => setDeleteTarget(null)}>Cancel</Button>
            <Button color="red" onClick={handleDelete} loading={actionLoading}>Delete</Button>
          </Group>
        </Stack>
      </Modal>

      {/* Bulk delete confirmation */}
      <Modal opened={bulkDeleteOpen} onClose={() => setBulkDeleteOpen(false)} title="Delete Admins">
        <Stack>
          <Text size="sm">
            Are you sure you want to delete <strong>{selection.count}</strong> admin(s)? This action cannot be undone.
          </Text>
          <Group justify="flex-end">
            <Button variant="default" onClick={() => setBulkDeleteOpen(false)}>Cancel</Button>
            <Button color="red" onClick={handleBulkDelete} loading={actionLoading}>Delete {selection.count} admin(s)</Button>
          </Group>
        </Stack>
      </Modal>
    </Stack>
  );
}
