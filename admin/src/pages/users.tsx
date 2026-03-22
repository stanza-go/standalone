import { useCallback, useEffect, useState } from "react";
import { useNavigate } from "react-router";
import {
  ActionIcon,
  Alert,
  Badge,
  Box,
  Button,
  Checkbox,
  Code,
  CopyButton,
  Group,
  Loader,
  Modal,
  Pagination,
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
  IconCopy,
  IconDownload,
  IconPencil,
  IconPlus,
  IconSearch,
  IconTrash,
  IconUserCheck,
  IconX,
} from "@tabler/icons-react";
import { del, downloadCSV, get, post, put, ApiError } from "@/lib/api";
import { useDebounce } from "@/hooks/use-debounce";
import { useSort } from "@/hooks/use-sort";
import { useSelection } from "@/hooks/use-selection";

interface User {
  id: number;
  email: string;
  name: string;
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

export default function UsersPage() {
  const navigate = useNavigate();
  const [users, setUsers] = useState<User[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [searchInput, setSearchInput] = useState("");
  const search = useDebounce(searchInput, 300);
  const [sort, toggleSort] = useSort("id", "desc");
  const selection = useSelection();
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  // Modal states
  const [createOpen, setCreateOpen] = useState(false);
  const [editUser, setEditUser] = useState<User | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<User | null>(null);
  const [bulkDeleteOpen, setBulkDeleteOpen] = useState(false);
  const [impersonateToken, setImpersonateToken] = useState("");
  const [actionLoading, setActionLoading] = useState(false);

  const createForm = useForm({
    initialValues: { email: "", password: "", name: "" },
    validate: {
      email: (v) => (!v ? "Email is required" : null),
      password: (v) => (!v ? "Password is required" : v.length < 8 ? "Minimum 8 characters" : null),
    },
  });

  const editForm = useForm({
    initialValues: { name: "", password: "" },
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
      const data = await get<{ users: User[]; total: number }>(`/admin/users?${params}`);
      setUsers(data.users ?? []);
      setTotal(data.total);
    } catch {
      setError("Failed to load users");
    } finally {
      setLoading(false);
    }
  }, [page, search, sort]);

  useEffect(() => {
    load();
  }, [load]);

  // Reset page when search changes
  useEffect(() => {
    setPage(1);
    selection.clear();
  }, [search]); // eslint-disable-line react-hooks/exhaustive-deps

  // Clear selection on page/sort change
  useEffect(() => {
    selection.clear();
  }, [page, sort]); // eslint-disable-line react-hooks/exhaustive-deps

  const totalPages = Math.ceil(total / PAGE_SIZE);

  async function handleCreate(values: typeof createForm.values) {
    setActionLoading(true);
    try {
      await post("/admin/users", values);
      notifications.show({ message: "User created", color: "green", icon: <IconCheck size={16} /> });
      setCreateOpen(false);
      createForm.reset();
      load();
    } catch (e) {
      if (e instanceof ApiError && Object.keys(e.fields).length > 0) {
        createForm.setErrors(e.fields);
      } else {
        notifications.show({ message: e instanceof ApiError ? e.message : "Failed to create user", color: "red" });
      }
    } finally {
      setActionLoading(false);
    }
  }

  function openEdit(user: User) {
    setEditUser(user);
    editForm.setValues({ name: user.name, password: "" });
  }

  async function handleEdit(values: typeof editForm.values) {
    if (!editUser) return;
    setActionLoading(true);
    try {
      const body: Record<string, unknown> = { name: values.name };
      if (values.password) body.password = values.password;
      await put(`/admin/users/${editUser.id}`, body);
      notifications.show({ message: "User updated", color: "green", icon: <IconCheck size={16} /> });
      setEditUser(null);
      load();
    } catch (e) {
      if (e instanceof ApiError && Object.keys(e.fields).length > 0) {
        editForm.setErrors(e.fields);
      } else {
        notifications.show({ message: e instanceof ApiError ? e.message : "Failed to update user", color: "red" });
      }
    } finally {
      setActionLoading(false);
    }
  }

  async function handleToggleActive(user: User) {
    try {
      await put(`/admin/users/${user.id}`, { is_active: !user.is_active });
      notifications.show({
        message: `User ${user.is_active ? "deactivated" : "activated"}`,
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
      await del(`/admin/users/${deleteTarget.id}`);
      notifications.show({ message: "User deleted", color: "green", icon: <IconCheck size={16} /> });
      setDeleteTarget(null);
      load();
    } catch {
      notifications.show({ message: "Failed to delete user", color: "red" });
    } finally {
      setActionLoading(false);
    }
  }

  async function handleBulkDelete() {
    setActionLoading(true);
    try {
      const data = await post<{ affected: number }>("/admin/users/bulk-delete", { ids: selection.ids });
      notifications.show({ message: `${data.affected} user(s) deleted`, color: "green", icon: <IconCheck size={16} /> });
      setBulkDeleteOpen(false);
      selection.clear();
      load();
    } catch {
      notifications.show({ message: "Failed to delete users", color: "red" });
    } finally {
      setActionLoading(false);
    }
  }

  async function handleImpersonate(user: User) {
    try {
      const data = await post<{ token: string }>(`/admin/users/${user.id}/impersonate`);
      setImpersonateToken(data.token);
    } catch {
      notifications.show({ message: "Failed to impersonate user", color: "red" });
    }
  }

  async function handleExport() {
    try {
      const params = new URLSearchParams({ sort: sort.column, order: sort.direction.toUpperCase() });
      if (search) params.set("search", search);
      await downloadCSV(`/admin/users/export?${params}`);
    } catch {
      notifications.show({ message: "Failed to export", color: "red" });
    }
  }

  const userIds = users.map((u) => u.id);

  return (
    <Stack>
      {/* Header */}
      <Group justify="space-between">
        <Group gap="xs">
          <Title order={3}>Users</Title>
          {!loading && <Badge variant="light" size="lg">{total}</Badge>}
        </Group>
        <Group gap="xs">
          <Button variant="subtle" size="xs" leftSection={<IconDownload size={16} />} onClick={handleExport}>
            Export CSV
          </Button>
          <Button leftSection={<IconPlus size={16} />} onClick={() => setCreateOpen(true)}>
            Create User
          </Button>
        </Group>
      </Group>

      {/* Search */}
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

      {/* Error */}
      {error && (
        <Alert icon={<IconAlertCircle size={16} />} color="red" variant="light" withCloseButton onClose={() => setError("")}>
          {error}
          <Button variant="subtle" size="xs" ml="sm" onClick={load}>Retry</Button>
        </Alert>
      )}

      {/* Table */}
      {loading && users.length === 0 ? (
        <Group justify="center" pt="xl">
          <Loader />
        </Group>
      ) : (
        <>
          <Table.ScrollContainer minWidth={600}>
            <Table>
              <Table.Thead>
                <Table.Tr>
                  <Table.Th w={40}>
                    <Checkbox
                      checked={selection.isAllSelected(userIds)}
                      indeterminate={selection.count > 0 && !selection.isAllSelected(userIds)}
                      onChange={() => selection.toggleAll(userIds)}
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
                  <Table.Th style={{ cursor: "pointer" }} onClick={() => toggleSort("is_active")}>
                    <Group gap={4} wrap="nowrap">Status <SortIcon column="is_active" sort={sort} /></Group>
                  </Table.Th>
                  <Table.Th style={{ cursor: "pointer" }} onClick={() => toggleSort("created_at")}>
                    <Group gap={4} wrap="nowrap">Created <SortIcon column="created_at" sort={sort} /></Group>
                  </Table.Th>
                  <Table.Th ta="right">Actions</Table.Th>
                </Table.Tr>
              </Table.Thead>
              <Table.Tbody>
                {users.length === 0 ? (
                  <Table.Tr>
                    <Table.Td colSpan={7}>
                      <Text ta="center" c="dimmed" py="lg">
                        {search ? "No users match your search" : "No users yet"}
                      </Text>
                    </Table.Td>
                  </Table.Tr>
                ) : (
                  users.map((user) => (
                    <Table.Tr
                      key={user.id}
                      bg={selection.isSelected(user.id) ? "var(--mantine-primary-color-light)" : undefined}
                    >
                      <Table.Td>
                        <Checkbox
                          checked={selection.isSelected(user.id)}
                          onChange={() => selection.toggle(user.id)}
                          aria-label={`Select ${user.email}`}
                        />
                      </Table.Td>
                      <Table.Td>
                        <Text size="sm" c="dimmed">{user.id}</Text>
                      </Table.Td>
                      <Table.Td>
                        <Text
                          size="sm"
                          fw={500}
                          style={{ cursor: "pointer" }}
                          c="blue"
                          onClick={() => navigate(`/users/${user.id}`)}
                        >
                          {user.email}
                        </Text>
                      </Table.Td>
                      <Table.Td>
                        <Text size="sm">{user.name || "—"}</Text>
                      </Table.Td>
                      <Table.Td>
                        <Badge
                          variant="light"
                          color={user.is_active ? "green" : "red"}
                          style={{ cursor: "pointer" }}
                          onClick={() => handleToggleActive(user)}
                        >
                          {user.is_active ? "Active" : "Inactive"}
                        </Badge>
                      </Table.Td>
                      <Table.Td>
                        <Tooltip label={new Date(user.created_at).toLocaleString()}>
                          <Text size="sm" c="dimmed">{timeAgo(user.created_at)}</Text>
                        </Tooltip>
                      </Table.Td>
                      <Table.Td>
                        <Group gap={4} justify="flex-end" wrap="nowrap">
                          <Tooltip label="Impersonate">
                            <ActionIcon variant="subtle" size="sm" onClick={() => handleImpersonate(user)}>
                              <IconUserCheck size={16} />
                            </ActionIcon>
                          </Tooltip>
                          <Tooltip label="Edit">
                            <ActionIcon variant="subtle" size="sm" onClick={() => openEdit(user)}>
                              <IconPencil size={16} />
                            </ActionIcon>
                          </Tooltip>
                          <Tooltip label="Delete">
                            <ActionIcon variant="subtle" size="sm" color="red" onClick={() => setDeleteTarget(user)}>
                              <IconTrash size={16} />
                            </ActionIcon>
                          </Tooltip>
                        </Group>
                      </Table.Td>
                    </Table.Tr>
                  ))
                )}
              </Table.Tbody>
            </Table>
          </Table.ScrollContainer>

          {/* Pagination */}
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

      {/* Bulk action bar */}
      {selection.count > 0 && (
        <Box
          pos="fixed"
          bottom={20}
          left="50%"
          style={{ transform: "translateX(-50%)", zIndex: 100 }}
        >
          <Group
            gap="sm"
            px="md"
            py="xs"
            style={(theme) => ({
              background: "var(--mantine-color-body)",
              border: `1px solid var(--mantine-color-default-border)`,
              borderRadius: theme.radius.md,
              boxShadow: theme.shadows.lg,
            })}
          >
            <Text size="sm" fw={500}>{selection.count} selected</Text>
            <Button
              variant="light"
              color="red"
              size="xs"
              leftSection={<IconTrash size={14} />}
              onClick={() => setBulkDeleteOpen(true)}
            >
              Delete
            </Button>
            <ActionIcon variant="subtle" size="sm" onClick={selection.clear}>
              <IconX size={14} />
            </ActionIcon>
          </Group>
        </Box>
      )}

      {/* Create modal */}
      <Modal opened={createOpen} onClose={() => setCreateOpen(false)} title="Create User">
        <form onSubmit={createForm.onSubmit(handleCreate)}>
          <Stack>
            <TextInput label="Email" placeholder="user@example.com" required {...createForm.getInputProps("email")} />
            <TextInput label="Password" type="password" placeholder="Minimum 8 characters" required {...createForm.getInputProps("password")} />
            <TextInput label="Name" placeholder="Full name" {...createForm.getInputProps("name")} />
            <Group justify="flex-end">
              <Button variant="default" onClick={() => setCreateOpen(false)}>Cancel</Button>
              <Button type="submit" loading={actionLoading}>Create</Button>
            </Group>
          </Stack>
        </form>
      </Modal>

      {/* Edit modal */}
      <Modal opened={!!editUser} onClose={() => setEditUser(null)} title="Edit User">
        <form onSubmit={editForm.onSubmit(handleEdit)}>
          <Stack>
            <TextInput label="Email" value={editUser?.email ?? ""} disabled />
            <TextInput label="Name" placeholder="Full name" {...editForm.getInputProps("name")} />
            <TextInput
              label="Password"
              type="password"
              placeholder="Leave empty to keep current"
              {...editForm.getInputProps("password")}
            />
            <Group justify="flex-end">
              <Button variant="default" onClick={() => setEditUser(null)}>Cancel</Button>
              <Button type="submit" loading={actionLoading}>Save</Button>
            </Group>
          </Stack>
        </form>
      </Modal>

      {/* Delete confirmation */}
      <Modal opened={!!deleteTarget} onClose={() => setDeleteTarget(null)} title="Delete User">
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
      <Modal opened={bulkDeleteOpen} onClose={() => setBulkDeleteOpen(false)} title="Delete Users">
        <Stack>
          <Text size="sm">
            Are you sure you want to delete <strong>{selection.count}</strong> user(s)? This action cannot be undone.
          </Text>
          <Group justify="flex-end">
            <Button variant="default" onClick={() => setBulkDeleteOpen(false)}>Cancel</Button>
            <Button color="red" onClick={handleBulkDelete} loading={actionLoading}>Delete {selection.count} user(s)</Button>
          </Group>
        </Stack>
      </Modal>

      {/* Impersonate token modal */}
      <Modal opened={!!impersonateToken} onClose={() => setImpersonateToken("")} title="Impersonation Token">
        <Stack>
          <Text size="sm" c="dimmed">
            Use this short-lived token to authenticate as this user. It will expire automatically.
          </Text>
          <Code block style={{ wordBreak: "break-all" }}>{impersonateToken}</Code>
          <Group justify="flex-end">
            <CopyButton value={impersonateToken}>
              {({ copied, copy }) => (
                <Button
                  variant={copied ? "light" : "default"}
                  color={copied ? "green" : undefined}
                  leftSection={copied ? <IconCheck size={16} /> : <IconCopy size={16} />}
                  onClick={copy}
                >
                  {copied ? "Copied" : "Copy Token"}
                </Button>
              )}
            </CopyButton>
          </Group>
        </Stack>
      </Modal>
    </Stack>
  );
}
