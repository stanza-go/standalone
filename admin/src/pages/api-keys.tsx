import { useCallback, useEffect, useState } from "react";
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
  IconBan,
  IconCheck,
  IconClock,
  IconCopy,
  IconDownload,
  IconPencil,
  IconPlus,
  IconSearch,
  IconTrash,
  IconX,
} from "@tabler/icons-react";
import { del, downloadCSV, get, post, put, ApiError } from "@/lib/api";
import { useDebounce } from "@/hooks/use-debounce";
import { useSort } from "@/hooks/use-sort";
import { useSelection } from "@/hooks/use-selection";
import { useTableKeyboard } from "@/hooks/use-table-keyboard";

interface ApiKey {
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

const PAGE_SIZE = 20;

function SortIcon({ column, sort }: { column: string; sort: { column: string; direction: string } }) {
  if (sort.column !== column) return <IconArrowsSort size={14} stroke={1.5} />;
  return sort.direction === "asc" ? <IconArrowUp size={14} stroke={1.5} /> : <IconArrowDown size={14} stroke={1.5} />;
}

function timeAgo(dateStr: string): string {
  if (!dateStr) return "—";
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

function keyStatus(key: ApiKey): { label: string; color: string; icon: React.ReactNode } {
  if (key.revoked_at) return { label: "Revoked", color: "red", icon: <IconBan size={10} /> };
  if (key.expires_at && new Date(key.expires_at) < new Date()) return { label: "Expired", color: "gray", icon: <IconClock size={10} /> };
  return { label: "Active", color: "green", icon: <IconCheck size={10} /> };
}

export default function ApiKeysPage() {
  const [keys, setKeys] = useState<ApiKey[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [searchInput, setSearchInput] = useState("");
  const search = useDebounce(searchInput, 300);
  const [sort, toggleSort] = useSort("id", "desc");
  const selection = useSelection();
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  const [createOpen, setCreateOpen] = useState(false);
  const [newKey, setNewKey] = useState("");
  const [editKey, setEditKey] = useState<ApiKey | null>(null);
  const [revokeTarget, setRevokeTarget] = useState<ApiKey | null>(null);
  const [bulkRevokeOpen, setBulkRevokeOpen] = useState(false);
  const [actionLoading, setActionLoading] = useState(false);

  const createForm = useForm({
    initialValues: { name: "", scopes: "", expires_at: "" },
    validate: {
      name: (v) => (!v ? "Name is required" : null),
    },
  });

  const editForm = useForm({
    initialValues: { name: "", scopes: "" },
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
      const data = await get<{ api_keys: ApiKey[]; total: number }>(`/admin/api-keys?${params}`);
      setKeys(data.api_keys ?? []);
      setTotal(data.total);
    } catch {
      setError("Failed to load API keys");
    } finally {
      setLoading(false);
    }
  }, [page, search, sort]);

  useEffect(() => { load(); }, [load]);
  useEffect(() => { setPage(1); selection.clear(); }, [search]); // eslint-disable-line react-hooks/exhaustive-deps
  useEffect(() => { selection.clear(); }, [page, sort]); // eslint-disable-line react-hooks/exhaustive-deps

  const totalPages = Math.ceil(total / PAGE_SIZE);

  async function handleCreate(values: typeof createForm.values) {
    setActionLoading(true);
    try {
      const body: Record<string, string> = { name: values.name };
      if (values.scopes) body.scopes = values.scopes;
      if (values.expires_at) body.expires_at = new Date(values.expires_at).toISOString();
      const data = await post<{ api_key: { key: string } }>("/admin/api-keys", body);
      setNewKey(data.api_key.key);
      setCreateOpen(false);
      createForm.reset();
      load();
    } catch (e) {
      if (e instanceof ApiError && Object.keys(e.fields).length > 0) {
        createForm.setErrors(e.fields);
      } else {
        notifications.show({ message: e instanceof ApiError ? e.message : "Failed to create API key", color: "red" });
      }
    } finally {
      setActionLoading(false);
    }
  }

  function openEdit(key: ApiKey) {
    setEditKey(key);
    editForm.setValues({ name: key.name, scopes: key.scopes });
  }

  async function handleEdit(values: typeof editForm.values) {
    if (!editKey) return;
    setActionLoading(true);
    try {
      await put(`/admin/api-keys/${editKey.id}`, { name: values.name, scopes: values.scopes });
      notifications.show({ message: "API key updated", color: "green", icon: <IconCheck size={16} /> });
      setEditKey(null);
      load();
    } catch (e) {
      notifications.show({ message: e instanceof ApiError ? e.message : "Failed to update", color: "red" });
    } finally {
      setActionLoading(false);
    }
  }

  async function handleRevoke() {
    if (!revokeTarget) return;
    setActionLoading(true);
    try {
      await del(`/admin/api-keys/${revokeTarget.id}`);
      notifications.show({ message: "API key revoked", color: "green", icon: <IconCheck size={16} /> });
      setRevokeTarget(null);
      load();
    } catch {
      notifications.show({ message: "Failed to revoke", color: "red" });
    } finally {
      setActionLoading(false);
    }
  }

  async function handleBulkRevoke() {
    setActionLoading(true);
    try {
      const data = await post<{ affected: number }>("/admin/api-keys/bulk-revoke", { ids: selection.ids });
      notifications.show({ message: `${data.affected} key(s) revoked`, color: "green", icon: <IconCheck size={16} /> });
      setBulkRevokeOpen(false);
      selection.clear();
      load();
    } catch {
      notifications.show({ message: "Failed to revoke keys", color: "red" });
    } finally {
      setActionLoading(false);
    }
  }

  async function handleExport() {
    try {
      const params = new URLSearchParams({ sort: sort.column, order: sort.direction.toUpperCase() });
      if (search) params.set("search", search);
      await downloadCSV(`/admin/api-keys/export?${params}`);
    } catch {
      notifications.show({ message: "Failed to export", color: "red" });
    }
  }

  const keyIds = keys.map((k) => k.id);

  const tableKeyboard = useTableKeyboard({
    rowCount: keys.length,
    onSelect: (i) => { const k = keys[i]; if (k) selection.toggle(k.id); },
  });

  return (
    <Stack>
      <Group justify="space-between" wrap="wrap">
        <Group gap="xs">
          <Title order={3}>API Keys</Title>
          {!loading && <Badge variant="light" size="lg">{total}</Badge>}
        </Group>
        <Group gap="xs">
          <Button variant="subtle" size="xs" leftSection={<IconDownload size={16} />} onClick={handleExport}>Export CSV</Button>
          <Button leftSection={<IconPlus size={16} />} onClick={() => setCreateOpen(true)}>Create Key</Button>
        </Group>
      </Group>

      <TextInput
        placeholder="Search by name or key prefix..."
        leftSection={<IconSearch size={16} />}
        value={searchInput}
        onChange={(e) => setSearchInput(e.currentTarget.value)}
        rightSection={searchInput ? <ActionIcon variant="subtle" size="sm" onClick={() => setSearchInput("")}><IconX size={14} /></ActionIcon> : null}
      />

      {error && (
        <Alert icon={<IconAlertCircle size={16} />} color="red" variant="light" withCloseButton onClose={() => setError("")}>
          {error}
          <Button variant="subtle" size="xs" ml="sm" onClick={load}>Retry</Button>
        </Alert>
      )}

      {loading && keys.length === 0 ? (
        <Group justify="center" pt="xl"><Loader /></Group>
      ) : (
        <>
          <Table.ScrollContainer minWidth={800}>
            <Table>
              <Table.Thead>
                <Table.Tr>
                  <Table.Th w={40}>
                    <Checkbox
                      checked={selection.isAllSelected(keyIds)}
                      indeterminate={selection.count > 0 && !selection.isAllSelected(keyIds)}
                      onChange={() => selection.toggleAll(keyIds)}
                      aria-label="Select all"
                    />
                  </Table.Th>
                  <Table.Th style={{ cursor: "pointer" }} onClick={() => toggleSort("name")}>
                    <Group gap={4} wrap="nowrap">Name <SortIcon column="name" sort={sort} /></Group>
                  </Table.Th>
                  <Table.Th>Key</Table.Th>
                  <Table.Th>Scopes</Table.Th>
                  <Table.Th style={{ cursor: "pointer" }} onClick={() => toggleSort("last_used_at")}>
                    <Group gap={4} wrap="nowrap">Last Used <SortIcon column="last_used_at" sort={sort} /></Group>
                  </Table.Th>
                  <Table.Th style={{ cursor: "pointer" }} onClick={() => toggleSort("request_count")}>
                    <Group gap={4} wrap="nowrap">Requests <SortIcon column="request_count" sort={sort} /></Group>
                  </Table.Th>
                  <Table.Th>Status</Table.Th>
                  <Table.Th style={{ cursor: "pointer" }} onClick={() => toggleSort("created_at")}>
                    <Group gap={4} wrap="nowrap">Created <SortIcon column="created_at" sort={sort} /></Group>
                  </Table.Th>
                  <Table.Th ta="right">Actions</Table.Th>
                </Table.Tr>
              </Table.Thead>
              <Table.Tbody {...tableKeyboard.tbodyProps}>
                {keys.length === 0 ? (
                  <Table.Tr>
                    <Table.Td colSpan={9}>
                      <Text ta="center" c="dimmed" py="lg">{search ? "No keys match your search" : "No API keys yet"}</Text>
                    </Table.Td>
                  </Table.Tr>
                ) : (
                  keys.map((key, idx) => {
                    const status = keyStatus(key);
                    return (
                      <Table.Tr key={key.id} bg={selection.isSelected(key.id) ? "var(--mantine-primary-color-light)" : undefined} style={tableKeyboard.isFocused(idx) ? { outline: "2px solid var(--mantine-primary-color-filled)", outlineOffset: -2 } : undefined}>
                        <Table.Td>
                          <Checkbox checked={selection.isSelected(key.id)} onChange={() => selection.toggle(key.id)} aria-label={`Select ${key.name}`} />
                        </Table.Td>
                        <Table.Td><Text size="sm" fw={500}>{key.name}</Text></Table.Td>
                        <Table.Td><Text size="sm" ff="monospace" c="dimmed">{key.key_prefix}...</Text></Table.Td>
                        <Table.Td>
                          {key.scopes ? (
                            <Group gap={4}>
                              {key.scopes.split(",").map((s) => (
                                <Badge key={s} variant="light" size="xs">{s.trim()}</Badge>
                              ))}
                            </Group>
                          ) : (
                            <Text size="sm" c="dimmed">All</Text>
                          )}
                        </Table.Td>
                        <Table.Td>
                          <Text size="sm" c="dimmed">{timeAgo(key.last_used_at)}</Text>
                        </Table.Td>
                        <Table.Td><Text size="sm">{key.request_count}</Text></Table.Td>
                        <Table.Td>
                          <Badge variant="light" color={status.color} size="sm" leftSection={status.icon}>{status.label}</Badge>
                        </Table.Td>
                        <Table.Td>
                          <Tooltip label={new Date(key.created_at).toLocaleString()}>
                            <Text size="sm" c="dimmed">{timeAgo(key.created_at)}</Text>
                          </Tooltip>
                        </Table.Td>
                        <Table.Td>
                          <Group gap={4} justify="flex-end" wrap="nowrap">
                            {!key.revoked_at && (
                              <>
                                <Tooltip label="Edit">
                                  <ActionIcon variant="subtle" size="sm" onClick={() => openEdit(key)}><IconPencil size={16} /></ActionIcon>
                                </Tooltip>
                                <Tooltip label="Revoke">
                                  <ActionIcon variant="subtle" size="sm" color="red" onClick={() => setRevokeTarget(key)}><IconTrash size={16} /></ActionIcon>
                                </Tooltip>
                              </>
                            )}
                          </Group>
                        </Table.Td>
                      </Table.Tr>
                    );
                  })
                )}
              </Table.Tbody>
            </Table>
          </Table.ScrollContainer>

          {totalPages > 1 && (
            <Group justify="space-between">
              <Text size="sm" c="dimmed">Showing {(page - 1) * PAGE_SIZE + 1}–{Math.min(page * PAGE_SIZE, total)} of {total}</Text>
              <Pagination value={page} onChange={setPage} total={totalPages} size="sm" />
            </Group>
          )}
        </>
      )}

      {selection.count > 0 && (
        <Box pos="fixed" bottom={20} left="50%" style={{ transform: "translateX(-50%)", zIndex: 100 }}>
          <Group gap="sm" px="md" py="xs" style={(theme) => ({ background: "var(--mantine-color-body)", border: "1px solid var(--mantine-color-default-border)", borderRadius: theme.radius.md, boxShadow: theme.shadows.lg })}>
            <Text size="sm" fw={500}>{selection.count} selected</Text>
            <Button variant="light" color="red" size="xs" leftSection={<IconTrash size={14} />} onClick={() => setBulkRevokeOpen(true)}>Revoke</Button>
            <ActionIcon variant="subtle" size="sm" onClick={selection.clear}><IconX size={14} /></ActionIcon>
          </Group>
        </Box>
      )}

      {/* Create modal */}
      <Modal opened={createOpen} onClose={() => setCreateOpen(false)} title="Create API Key">
        <form onSubmit={createForm.onSubmit(handleCreate)}>
          <Stack>
            <TextInput label="Name" placeholder="My API Key" required {...createForm.getInputProps("name")} />
            <TextInput label="Scopes" placeholder="Comma-separated (empty = all)" {...createForm.getInputProps("scopes")} />
            <TextInput label="Expires" type="datetime-local" {...createForm.getInputProps("expires_at")} />
            <Group justify="flex-end">
              <Button variant="default" onClick={() => setCreateOpen(false)}>Cancel</Button>
              <Button type="submit" loading={actionLoading}>Create</Button>
            </Group>
          </Stack>
        </form>
      </Modal>

      {/* New key display */}
      <Modal opened={!!newKey} onClose={() => setNewKey("")} title="API Key Created">
        <Stack>
          <Alert color="yellow" variant="light">Copy this key now — it will not be shown again.</Alert>
          <Code block style={{ wordBreak: "break-all" }}>{newKey}</Code>
          <Group justify="flex-end">
            <CopyButton value={newKey}>
              {({ copied, copy }) => (
                <Button variant={copied ? "light" : "default"} color={copied ? "green" : undefined} leftSection={copied ? <IconCheck size={16} /> : <IconCopy size={16} />} onClick={copy}>
                  {copied ? "Copied" : "Copy Key"}
                </Button>
              )}
            </CopyButton>
          </Group>
        </Stack>
      </Modal>

      {/* Edit modal */}
      <Modal opened={!!editKey} onClose={() => setEditKey(null)} title="Edit API Key">
        <form onSubmit={editForm.onSubmit(handleEdit)}>
          <Stack>
            <TextInput label="Name" {...editForm.getInputProps("name")} />
            <TextInput label="Scopes" placeholder="Comma-separated (empty = all)" {...editForm.getInputProps("scopes")} />
            <Group justify="flex-end">
              <Button variant="default" onClick={() => setEditKey(null)}>Cancel</Button>
              <Button type="submit" loading={actionLoading}>Save</Button>
            </Group>
          </Stack>
        </form>
      </Modal>

      {/* Revoke confirmation */}
      <Modal opened={!!revokeTarget} onClose={() => setRevokeTarget(null)} title="Revoke API Key" size="sm">
        <Stack>
          <Text size="sm">Revoke API key <strong>{revokeTarget?.name}</strong>? This cannot be undone.</Text>
          <Group justify="flex-end">
            <Button variant="default" onClick={() => setRevokeTarget(null)}>Cancel</Button>
            <Button color="red" onClick={handleRevoke} loading={actionLoading}>Revoke</Button>
          </Group>
        </Stack>
      </Modal>

      {/* Bulk revoke */}
      <Modal opened={bulkRevokeOpen} onClose={() => setBulkRevokeOpen(false)} title="Revoke API Keys" size="sm">
        <Stack>
          <Text size="sm">Revoke <strong>{selection.count}</strong> API key(s)? This cannot be undone.</Text>
          <Group justify="flex-end">
            <Button variant="default" onClick={() => setBulkRevokeOpen(false)}>Cancel</Button>
            <Button color="red" onClick={handleBulkRevoke} loading={actionLoading}>Revoke {selection.count} key(s)</Button>
          </Group>
        </Stack>
      </Modal>
    </Stack>
  );
}
