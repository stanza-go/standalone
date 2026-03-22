import { useCallback, useEffect, useState } from "react";
import {
  ActionIcon,
  Alert,
  Badge,
  Box,
  Button,
  Checkbox,
  Group,
  Indicator,
  Loader,
  Modal,
  NativeSelect,
  Pagination,
  Stack,
  Table,
  Text,
  Title,
  Tooltip,
} from "@mantine/core";
import { notifications as notify } from "@mantine/notifications";
import {
  IconAlertCircle,
  IconArrowDown,
  IconArrowUp,
  IconArrowsSort,
  IconBell,
  IconCheck,
  IconChecks,
  IconDownload,
  IconFilter,
  IconTrash,
  IconX,
} from "@tabler/icons-react";
import { get, post, del, downloadCSV } from "@/lib/api";
import { useSort } from "@/hooks/use-sort";
import { useSelection } from "@/hooks/use-selection";

interface Notification {
  id: number;
  entity_type: string;
  entity_id: number;
  type: string;
  title: string;
  message: string;
  data?: string;
  read_at?: string;
  created_at: string;
}

interface ListResponse {
  notifications: Notification[];
  total: number;
  unread: number;
}

const PAGE_SIZE = 20;

const TYPE_COLORS: Record<string, string> = {
  info: "blue",
  success: "green",
  warning: "yellow",
  error: "red",
};

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

export default function NotificationsPage() {
  const [items, setItems] = useState<Notification[]>([]);
  const [total, setTotal] = useState(0);
  const [unread, setUnread] = useState(0);
  const [page, setPage] = useState(1);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [actionLoading, setActionLoading] = useState(false);

  // Filter
  const [unreadOnly, setUnreadOnly] = useState("all");

  const [sort, toggleSort] = useSort("id", "desc");
  const selection = useSelection();

  // Modal states
  const [deleteTarget, setDeleteTarget] = useState<Notification | null>(null);
  const [bulkDeleteOpen, setBulkDeleteOpen] = useState(false);

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
      if (unreadOnly === "unread") params.set("unread", "true");

      const data = await get<ListResponse>(`/admin/notifications?${params}`);
      setItems(data.notifications ?? []);
      setTotal(data.total);
      setUnread(data.unread);
    } catch {
      setError("Failed to load notifications");
    } finally {
      setLoading(false);
    }
  }, [page, unreadOnly, sort]);

  useEffect(() => {
    load();
  }, [load]);

  useEffect(() => {
    setPage(1);
    selection.clear();
  }, [unreadOnly]); // eslint-disable-line react-hooks/exhaustive-deps

  useEffect(() => {
    selection.clear();
  }, [page, sort]); // eslint-disable-line react-hooks/exhaustive-deps

  async function markRead(id: number) {
    setActionLoading(true);
    try {
      await post(`/admin/notifications/${id}/read`);
      setItems((prev) => prev.map((n) => (n.id === id ? { ...n, read_at: new Date().toISOString() } : n)));
      setUnread((c) => Math.max(0, c - 1));
    } catch {
      notify.show({ message: "Failed to mark as read", color: "red" });
    } finally {
      setActionLoading(false);
    }
  }

  async function markAllRead() {
    setActionLoading(true);
    try {
      await post("/admin/notifications/read-all");
      setItems((prev) => prev.map((n) => ({ ...n, read_at: n.read_at || new Date().toISOString() })));
      setUnread(0);
      notify.show({ message: "All notifications marked as read", color: "green", icon: <IconCheck size={16} /> });
    } catch {
      notify.show({ message: "Failed to mark all as read", color: "red" });
    } finally {
      setActionLoading(false);
    }
  }

  async function handleDelete() {
    if (!deleteTarget) return;
    setActionLoading(true);
    try {
      await del(`/admin/notifications/${deleteTarget.id}`);
      notify.show({ message: "Notification deleted", color: "green", icon: <IconCheck size={16} /> });
      setDeleteTarget(null);
      load();
    } catch {
      notify.show({ message: "Failed to delete notification", color: "red" });
    } finally {
      setActionLoading(false);
    }
  }

  async function handleBulkDelete() {
    setActionLoading(true);
    try {
      const data = await post<{ affected: number }>("/admin/notifications/bulk-delete", { ids: selection.ids });
      notify.show({ message: `${data.affected} notification(s) deleted`, color: "green", icon: <IconCheck size={16} /> });
      setBulkDeleteOpen(false);
      selection.clear();
      load();
    } catch {
      notify.show({ message: "Failed to delete notifications", color: "red" });
    } finally {
      setActionLoading(false);
    }
  }

  async function handleExport() {
    try {
      const params = new URLSearchParams({
        sort: sort.column,
        order: sort.direction.toUpperCase(),
      });
      if (unreadOnly === "unread") params.set("unread", "true");
      await downloadCSV(`/admin/notifications/export?${params}`);
    } catch {
      notify.show({ message: "Failed to export", color: "red" });
    }
  }

  const totalPages = Math.ceil(total / PAGE_SIZE);
  const itemIds = items.map((n) => n.id);

  return (
    <Stack>
      {/* Header */}
      <Group justify="space-between" wrap="wrap">
        <Group gap="xs">
          <Title order={3}>Notifications</Title>
          {!loading && (
            <Group gap={4}>
              <Badge variant="light" size="lg">{total}</Badge>
              {unread > 0 && <Badge variant="light" color="red" size="lg">{unread} unread</Badge>}
            </Group>
          )}
        </Group>
        <Group gap="xs">
          <Button variant="subtle" size="xs" leftSection={<IconDownload size={16} />} onClick={handleExport}>
            Export CSV
          </Button>
          {unread > 0 && (
            <Button
              variant="light"
              size="xs"
              leftSection={<IconChecks size={16} />}
              onClick={markAllRead}
              loading={actionLoading}
            >
              Mark all read
            </Button>
          )}
        </Group>
      </Group>

      {/* Filter */}
      <Group gap="sm">
        <NativeSelect
          leftSection={<IconFilter size={16} />}
          value={unreadOnly}
          onChange={(e) => setUnreadOnly(e.currentTarget.value)}
          data={[
            { value: "all", label: "All notifications" },
            { value: "unread", label: "Unread only" },
          ]}
          w={200}
        />
        {unreadOnly === "unread" && (
          <Button variant="subtle" size="xs" leftSection={<IconX size={14} />} onClick={() => setUnreadOnly("all")}>
            Clear filter
          </Button>
        )}
      </Group>

      {/* Error */}
      {error && (
        <Alert icon={<IconAlertCircle size={16} />} color="red" variant="light" withCloseButton onClose={() => setError("")}>
          {error}
          <Button variant="subtle" size="xs" ml="sm" onClick={load}>Retry</Button>
        </Alert>
      )}

      {/* Table */}
      {loading && items.length === 0 ? (
        <Group justify="center" pt="xl">
          <Loader />
        </Group>
      ) : (
        <>
          <Table.ScrollContainer minWidth={500}>
            <Table>
              <Table.Thead>
                <Table.Tr>
                  <Table.Th w={40}>
                    <Checkbox
                      checked={selection.isAllSelected(itemIds)}
                      indeterminate={selection.count > 0 && !selection.isAllSelected(itemIds)}
                      onChange={() => selection.toggleAll(itemIds)}
                      aria-label="Select all"
                    />
                  </Table.Th>
                  <Table.Th w={32}></Table.Th>
                  <Table.Th>Notification</Table.Th>
                  <Table.Th style={{ cursor: "pointer" }} onClick={() => toggleSort("type")}>
                    <Group gap={4} wrap="nowrap">Type <SortIcon column="type" sort={sort} /></Group>
                  </Table.Th>
                  <Table.Th style={{ cursor: "pointer" }} onClick={() => toggleSort("created_at")}>
                    <Group gap={4} wrap="nowrap">Time <SortIcon column="created_at" sort={sort} /></Group>
                  </Table.Th>
                  <Table.Th ta="right">Actions</Table.Th>
                </Table.Tr>
              </Table.Thead>
              <Table.Tbody>
                {items.length === 0 ? (
                  <Table.Tr>
                    <Table.Td colSpan={6}>
                      <Text ta="center" c="dimmed" py="lg">
                        {unreadOnly === "unread" ? "No unread notifications" : "No notifications yet"}
                      </Text>
                    </Table.Td>
                  </Table.Tr>
                ) : (
                  items.map((n) => (
                    <Table.Tr
                      key={n.id}
                      bg={
                        selection.isSelected(n.id)
                          ? "var(--mantine-primary-color-light)"
                          : !n.read_at
                          ? "var(--mantine-color-default-hover)"
                          : undefined
                      }
                    >
                      <Table.Td>
                        <Checkbox
                          checked={selection.isSelected(n.id)}
                          onChange={() => selection.toggle(n.id)}
                          aria-label={`Select notification ${n.id}`}
                        />
                      </Table.Td>
                      <Table.Td>
                        {!n.read_at ? (
                          <Indicator color={TYPE_COLORS[n.type] || "blue"} size={10} processing>
                            <span />
                          </Indicator>
                        ) : (
                          <IconBell size={14} style={{ opacity: 0.3 }} />
                        )}
                      </Table.Td>
                      <Table.Td>
                        <Text size="sm" fw={n.read_at ? 400 : 600}>{n.title}</Text>
                        <Text size="xs" c="dimmed" lineClamp={2}>{n.message}</Text>
                      </Table.Td>
                      <Table.Td>
                        <Badge variant="light" color={TYPE_COLORS[n.type] || "gray"} size="sm">{n.type}</Badge>
                      </Table.Td>
                      <Table.Td>
                        <Tooltip label={new Date(n.created_at).toLocaleString()}>
                          <Text size="sm" c="dimmed">{timeAgo(n.created_at)}</Text>
                        </Tooltip>
                        {n.read_at && (
                          <Text size="xs" c="dimmed" style={{ opacity: 0.6 }}>Read {timeAgo(n.read_at)}</Text>
                        )}
                      </Table.Td>
                      <Table.Td>
                        <Group gap={4} justify="flex-end" wrap="nowrap">
                          {!n.read_at && (
                            <Tooltip label="Mark as read">
                              <ActionIcon variant="subtle" size="sm" onClick={() => markRead(n.id)}>
                                <IconCheck size={16} />
                              </ActionIcon>
                            </Tooltip>
                          )}
                          <Tooltip label="Delete">
                            <ActionIcon variant="subtle" size="sm" color="red" onClick={() => setDeleteTarget(n)}>
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
        <Box pos="fixed" bottom={20} left="50%" style={{ transform: "translateX(-50%)", zIndex: 100 }}>
          <Group
            gap="sm"
            px="md"
            py="xs"
            style={(theme) => ({
              background: "var(--mantine-color-body)",
              border: "1px solid var(--mantine-color-default-border)",
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

      {/* Delete confirmation */}
      <Modal opened={!!deleteTarget} onClose={() => setDeleteTarget(null)} title="Delete Notification">
        <Stack>
          <Text size="sm">Are you sure you want to delete this notification?</Text>
          {deleteTarget && (
            <Box>
              <Text size="sm" fw={500}>{deleteTarget.title}</Text>
              <Text size="xs" c="dimmed" lineClamp={2}>{deleteTarget.message}</Text>
            </Box>
          )}
          <Group justify="flex-end">
            <Button variant="default" onClick={() => setDeleteTarget(null)}>Cancel</Button>
            <Button color="red" onClick={handleDelete} loading={actionLoading}>Delete</Button>
          </Group>
        </Stack>
      </Modal>

      {/* Bulk delete confirmation */}
      <Modal opened={bulkDeleteOpen} onClose={() => setBulkDeleteOpen(false)} title="Delete Notifications">
        <Stack>
          <Text size="sm">
            Are you sure you want to delete <strong>{selection.count}</strong> notification(s)? This action cannot be undone.
          </Text>
          <Group justify="flex-end">
            <Button variant="default" onClick={() => setBulkDeleteOpen(false)}>Cancel</Button>
            <Button color="red" onClick={handleBulkDelete} loading={actionLoading}>
              Delete {selection.count} notification(s)
            </Button>
          </Group>
        </Stack>
      </Modal>
    </Stack>
  );
}
