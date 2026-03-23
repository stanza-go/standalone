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
  Switch,
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
  IconExternalLink,
  IconPencil,
  IconPlayerPause,
  IconPlus,
  IconSearch,
  IconTrash,
  IconWebhook,
  IconX,
} from "@tabler/icons-react";
import { get, post, put, del, downloadCSV, ApiError } from "@/lib/api";
import { EmptyState } from "@/components/empty-state";
import { useDebounce } from "@/hooks/use-debounce";
import { useSort } from "@/hooks/use-sort";
import { useSelection } from "@/hooks/use-selection";
import { useTableKeyboard } from "@/hooks/use-table-keyboard";

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

export default function WebhooksPage() {
  const navigate = useNavigate();
  const [webhooks, setWebhooks] = useState<Webhook[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [searchInput, setSearchInput] = useState("");
  const search = useDebounce(searchInput, 300);
  const [sort, toggleSort] = useSort("created_at", "desc");
  const selection = useSelection();
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [actionLoading, setActionLoading] = useState(false);

  // Modal states
  const [createOpen, setCreateOpen] = useState(false);
  const [editWebhook, setEditWebhook] = useState<Webhook | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<Webhook | null>(null);
  const [bulkDeleteOpen, setBulkDeleteOpen] = useState(false);
  const [createdWebhook, setCreatedWebhook] = useState<Webhook | null>(null);

  const createForm = useForm({
    initialValues: { url: "", description: "", events: "*" },
    validate: {
      url: (v) => (!v ? "URL is required" : null),
    },
  });

  const editForm = useForm({
    initialValues: { url: "", description: "", events: "", is_active: true },
    validate: {
      url: (v) => (!v ? "URL is required" : null),
    },
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
      const data = await get<{ webhooks: Webhook[]; total: number }>(`/admin/webhooks?${params}`);
      setWebhooks(data.webhooks ?? []);
      setTotal(data.total);
    } catch {
      setError("Failed to load webhooks");
    } finally {
      setLoading(false);
    }
  }, [page, search, sort]);

  useEffect(() => {
    load();
    const interval = setInterval(load, 30000);
    return () => clearInterval(interval);
  }, [load]);

  useEffect(() => {
    setPage(1);
    selection.clear();
  }, [search]); // eslint-disable-line react-hooks/exhaustive-deps

  useEffect(() => {
    selection.clear();
  }, [page, sort]); // eslint-disable-line react-hooks/exhaustive-deps

  function openEdit(wh: Webhook) {
    setEditWebhook(wh);
    editForm.setValues({
      url: wh.url,
      description: wh.description,
      events: wh.events.join(", "),
      is_active: wh.is_active,
    });
  }

  async function handleCreate(values: typeof createForm.values) {
    setActionLoading(true);
    try {
      const eventsList = values.events.split(",").map((s) => s.trim()).filter(Boolean);
      const data = await post<Webhook>("/admin/webhooks", {
        url: values.url,
        description: values.description,
        events: eventsList,
      });
      notifications.show({ message: "Webhook created", color: "green", icon: <IconCheck size={16} /> });
      setCreateOpen(false);
      createForm.reset();
      setCreatedWebhook(data);
      load();
    } catch (e) {
      if (e instanceof ApiError && Object.keys(e.fields).length > 0) {
        createForm.setErrors(e.fields);
      } else {
        notifications.show({ message: e instanceof ApiError ? e.message : "Failed to create webhook", color: "red" });
      }
    } finally {
      setActionLoading(false);
    }
  }

  async function handleEdit(values: typeof editForm.values) {
    if (!editWebhook) return;
    setActionLoading(true);
    try {
      const eventsList = values.events.split(",").map((s) => s.trim()).filter(Boolean);
      await put(`/admin/webhooks/${editWebhook.id}`, {
        url: values.url,
        description: values.description,
        events: eventsList,
        is_active: values.is_active,
      });
      notifications.show({ message: "Webhook updated", color: "green", icon: <IconCheck size={16} /> });
      setEditWebhook(null);
      load();
    } catch (e) {
      if (e instanceof ApiError && Object.keys(e.fields).length > 0) {
        editForm.setErrors(e.fields);
      } else {
        notifications.show({ message: e instanceof ApiError ? e.message : "Failed to update webhook", color: "red" });
      }
    } finally {
      setActionLoading(false);
    }
  }

  async function handleDelete() {
    if (!deleteTarget) return;
    setActionLoading(true);
    try {
      await del(`/admin/webhooks/${deleteTarget.id}`);
      notifications.show({ message: "Webhook deleted", color: "green", icon: <IconCheck size={16} /> });
      setDeleteTarget(null);
      load();
    } catch {
      notifications.show({ message: "Failed to delete webhook", color: "red" });
    } finally {
      setActionLoading(false);
    }
  }

  async function handleBulkDelete() {
    setActionLoading(true);
    try {
      const data = await post<{ affected: number }>("/admin/webhooks/bulk-delete", { ids: selection.ids });
      notifications.show({ message: `${data.affected} webhook(s) deleted`, color: "green", icon: <IconCheck size={16} /> });
      setBulkDeleteOpen(false);
      selection.clear();
      load();
    } catch {
      notifications.show({ message: "Failed to delete webhooks", color: "red" });
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
      if (search) params.set("search", search);
      await downloadCSV(`/admin/webhooks/export?${params}`);
    } catch {
      notifications.show({ message: "Failed to export", color: "red" });
    }
  }

  const totalPages = Math.ceil(total / PAGE_SIZE);
  const webhookIds = webhooks.map((w) => w.id);

  const tableKeyboard = useTableKeyboard({
    rowCount: webhooks.length,
    onActivate: (i) => { const w = webhooks[i]; if (w) navigate(`/webhooks/${w.id}`); },
    onSelect: (i) => { const w = webhooks[i]; if (w) selection.toggle(w.id); },
  });

  return (
    <Stack>
      {/* Header */}
      <Group justify="space-between" wrap="wrap">
        <Group gap="xs">
          <Title order={3}>Webhooks</Title>
          {!loading && <Badge variant="light" size="lg">{total}</Badge>}
        </Group>
        <Group gap="xs">
          <Button variant="subtle" size="xs" leftSection={<IconDownload size={16} />} onClick={handleExport}>
            Export CSV
          </Button>
          <Button leftSection={<IconPlus size={16} />} onClick={() => setCreateOpen(true)}>
            Add Webhook
          </Button>
        </Group>
      </Group>

      {/* Search */}
      <TextInput
        placeholder="Search by URL or description..."
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

      {/* Secret reveal after creation */}
      {createdWebhook && (
        <Alert
          color="green"
          variant="light"
          title="Webhook created — signing secret"
          withCloseButton
          onClose={() => setCreatedWebhook(null)}
        >
          <Text size="xs" mb="xs">Copy this secret now. Use it to verify webhook signatures.</Text>
          <Group gap="xs">
            <Code block style={{ flex: 1, wordBreak: "break-all" }}>{createdWebhook.secret}</Code>
            <CopyButton value={createdWebhook.secret}>
              {({ copied, copy }) => (
                <ActionIcon
                  variant={copied ? "light" : "default"}
                  color={copied ? "green" : undefined}
                  onClick={copy}
                  size="lg"
                >
                  {copied ? <IconCheck size={16} /> : <IconCopy size={16} />}
                </ActionIcon>
              )}
            </CopyButton>
          </Group>
        </Alert>
      )}

      {/* Error */}
      {error && (
        <Alert icon={<IconAlertCircle size={16} />} color="red" variant="light" withCloseButton onClose={() => setError("")}>
          {error}
          <Button variant="subtle" size="xs" ml="sm" onClick={load}>Retry</Button>
        </Alert>
      )}

      {/* Table */}
      {loading && webhooks.length === 0 ? (
        <Group justify="center" pt="xl">
          <Loader />
        </Group>
      ) : webhooks.length === 0 ? (
        <EmptyState
          icon={<IconWebhook size={24} />}
          title={search ? "No webhooks match your search" : "No webhooks configured"}
          description={search ? "Try a different search term." : "Set up webhooks to notify external services of events."}
          action={!search ? <Button leftSection={<IconPlus size={16} />} onClick={() => setCreateOpen(true)}>Create Webhook</Button> : undefined}
        />
      ) : (
        <>
          <Table.ScrollContainer minWidth={600}>
            <Table>
              <Table.Thead>
                <Table.Tr>
                  <Table.Th w={40}>
                    <Checkbox
                      checked={selection.isAllSelected(webhookIds)}
                      indeterminate={selection.count > 0 && !selection.isAllSelected(webhookIds)}
                      onChange={() => selection.toggleAll(webhookIds)}
                      aria-label="Select all"
                    />
                  </Table.Th>
                  <Table.Th style={{ cursor: "pointer" }} onClick={() => toggleSort("url")}>
                    <Group gap={4} wrap="nowrap">URL <SortIcon column="url" sort={sort} /></Group>
                  </Table.Th>
                  <Table.Th>Description</Table.Th>
                  <Table.Th>Events</Table.Th>
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
                {webhooks.map((wh, idx) => (
                    <Table.Tr
                      key={wh.id}
                      bg={selection.isSelected(wh.id) ? "var(--mantine-primary-color-light)" : undefined}
                      style={{ cursor: "pointer", ...(tableKeyboard.isFocused(idx) ? { outline: "2px solid var(--mantine-primary-color-filled)", outlineOffset: -2 } : {}) }}
                      onClick={() => navigate(`/webhooks/${wh.id}`)}
                    >
                      <Table.Td onClick={(e) => e.stopPropagation()}>
                        <Checkbox
                          checked={selection.isSelected(wh.id)}
                          onChange={() => selection.toggle(wh.id)}
                          aria-label={`Select ${wh.url}`}
                        />
                      </Table.Td>
                      <Table.Td>
                        <Group gap={4} wrap="nowrap">
                          <Text size="xs" ff="monospace" truncate maw={280}>{wh.url}</Text>
                          <IconExternalLink size={12} style={{ opacity: 0.5 }} />
                        </Group>
                      </Table.Td>
                      <Table.Td>
                        <Text size="xs" c="dimmed">{wh.description || "\u2014"}</Text>
                      </Table.Td>
                      <Table.Td>
                        <Group gap={4} wrap="wrap">
                          {wh.events.map((ev) => (
                            <Badge
                              key={ev}
                              variant="light"
                              color={ev === "*" ? "violet" : "blue"}
                              size="xs"
                            >
                              {ev === "*" ? "all events" : ev}
                            </Badge>
                          ))}
                        </Group>
                      </Table.Td>
                      <Table.Td>
                        <Badge variant="light" color={wh.is_active ? "green" : "yellow"} size="sm" leftSection={wh.is_active ? <IconCheck size={10} /> : <IconPlayerPause size={10} />}>
                          {wh.is_active ? "Active" : "Paused"}
                        </Badge>
                      </Table.Td>
                      <Table.Td>
                        <Tooltip label={new Date(wh.created_at).toLocaleString()}>
                          <Text size="sm" c="dimmed">{timeAgo(wh.created_at)}</Text>
                        </Tooltip>
                      </Table.Td>
                      <Table.Td onClick={(e) => e.stopPropagation()}>
                        <Group gap={4} justify="flex-end" wrap="nowrap">
                          <Tooltip label="Edit">
                            <ActionIcon variant="subtle" size="sm" onClick={() => openEdit(wh)}>
                              <IconPencil size={16} />
                            </ActionIcon>
                          </Tooltip>
                          <Tooltip label="Delete">
                            <ActionIcon variant="subtle" size="sm" color="red" onClick={() => setDeleteTarget(wh)}>
                              <IconTrash size={16} />
                            </ActionIcon>
                          </Tooltip>
                        </Group>
                      </Table.Td>
                    </Table.Tr>
                  ))}
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

      {/* Create modal */}
      <Modal opened={createOpen} onClose={() => setCreateOpen(false)} title="Add Webhook">
        <form onSubmit={createForm.onSubmit(handleCreate)}>
          <Stack>
            <TextInput label="Endpoint URL" placeholder="https://example.com/webhook" required {...createForm.getInputProps("url")} />
            <TextInput label="Description" description="Optional" placeholder="e.g. Slack notification for new users" {...createForm.getInputProps("description")} />
            <TextInput label="Events" description="Comma-separated, * = all" placeholder="e.g. user.created, user.updated" {...createForm.getInputProps("events")} />
            <Group justify="flex-end">
              <Button variant="default" onClick={() => setCreateOpen(false)}>Cancel</Button>
              <Button type="submit" loading={actionLoading}>Add Webhook</Button>
            </Group>
          </Stack>
        </form>
      </Modal>

      {/* Edit modal */}
      <Modal opened={!!editWebhook} onClose={() => setEditWebhook(null)} title="Edit Webhook">
        <form onSubmit={editForm.onSubmit(handleEdit)}>
          <Stack>
            <TextInput label="Endpoint URL" placeholder="https://example.com/webhook" required {...editForm.getInputProps("url")} />
            <TextInput label="Description" description="Optional" placeholder="e.g. Slack notification for new users" {...editForm.getInputProps("description")} />
            <TextInput label="Events" description="Comma-separated, * = all" placeholder="e.g. user.created, user.updated" {...editForm.getInputProps("events")} />
            <Switch label="Active" {...editForm.getInputProps("is_active", { type: "checkbox" })} />
            <Group justify="flex-end">
              <Button variant="default" onClick={() => setEditWebhook(null)}>Cancel</Button>
              <Button type="submit" loading={actionLoading}>Save Changes</Button>
            </Group>
          </Stack>
        </form>
      </Modal>

      {/* Delete confirmation */}
      <Modal opened={!!deleteTarget} onClose={() => setDeleteTarget(null)} title="Delete Webhook">
        <Stack>
          <Text size="sm">Are you sure you want to delete this webhook? All delivery history will also be removed.</Text>
          {deleteTarget && (
            <Box>
              <Text size="sm"><strong>URL:</strong> <Text span size="xs" ff="monospace">{deleteTarget.url}</Text></Text>
              {deleteTarget.description && <Text size="sm"><strong>Description:</strong> {deleteTarget.description}</Text>}
            </Box>
          )}
          <Group justify="flex-end">
            <Button variant="default" onClick={() => setDeleteTarget(null)}>Cancel</Button>
            <Button color="red" onClick={handleDelete} loading={actionLoading}>Delete</Button>
          </Group>
        </Stack>
      </Modal>

      {/* Bulk delete confirmation */}
      <Modal opened={bulkDeleteOpen} onClose={() => setBulkDeleteOpen(false)} title="Delete Webhooks">
        <Stack>
          <Text size="sm">
            Are you sure you want to delete <strong>{selection.count}</strong> webhook(s)? This action cannot be undone.
          </Text>
          <Group justify="flex-end">
            <Button variant="default" onClick={() => setBulkDeleteOpen(false)}>Cancel</Button>
            <Button color="red" onClick={handleBulkDelete} loading={actionLoading}>
              Delete {selection.count} webhook(s)
            </Button>
          </Group>
        </Stack>
      </Modal>
    </Stack>
  );
}
