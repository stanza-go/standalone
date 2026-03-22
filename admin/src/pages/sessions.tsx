import { useCallback, useEffect, useState } from "react";
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
  Stack,
  Table,
  Text,
  Title,
  Tooltip,
} from "@mantine/core";
import { notifications } from "@mantine/notifications";
import {
  IconAlertCircle,
  IconArrowDown,
  IconArrowUp,
  IconArrowsSort,
  IconCheck,
  IconDownload,
  IconTrash,
  IconX,
} from "@tabler/icons-react";
import { del, downloadCSV, get, post } from "@/lib/api";
import { useSort } from "@/hooks/use-sort";
import { useTableKeyboard } from "@/hooks/use-table-keyboard";

interface Session {
  id: string;
  entity_type: string;
  entity_id: string;
  email: string;
  name: string;
  created_at: string;
  expires_at: string;
}

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

export default function SessionsPage() {
  const [sessions, setSessions] = useState<Session[]>([]);
  const [sort, toggleSort] = useSort("created_at", "desc");
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [revokeTarget, setRevokeTarget] = useState<Session | null>(null);
  const [bulkRevokeOpen, setBulkRevokeOpen] = useState(false);
  const [actionLoading, setActionLoading] = useState(false);

  const toggleSelect = useCallback((id: string) => {
    setSelected((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id); else next.add(id);
      return next;
    });
  }, []);

  const toggleAll = useCallback(() => {
    setSelected((prev) => {
      const ids = sessions.map((s) => s.id);
      const allSelected = ids.length > 0 && ids.every((id) => prev.has(id));
      return allSelected ? new Set() : new Set(ids);
    });
  }, [sessions]);

  const load = useCallback(async () => {
    setLoading(true);
    setError("");
    try {
      const params = new URLSearchParams({
        sort: sort.column,
        order: sort.direction.toUpperCase(),
      });
      const data = await get<{ sessions: Session[] }>(`/admin/sessions?${params}`);
      setSessions(data.sessions ?? []);
    } catch {
      setError("Failed to load sessions");
    } finally {
      setLoading(false);
    }
  }, [sort]);

  useEffect(() => {
    load();
  }, [load]);

  useEffect(() => {
    setSelected(new Set());
  }, [sort]);

  async function handleRevoke() {
    if (!revokeTarget) return;
    setActionLoading(true);
    try {
      await del(`/admin/sessions/${revokeTarget.id}`);
      notifications.show({ message: "Session revoked", color: "green", icon: <IconCheck size={16} /> });
      setRevokeTarget(null);
      load();
    } catch {
      notifications.show({ message: "Failed to revoke session", color: "red" });
    } finally {
      setActionLoading(false);
    }
  }

  async function handleBulkRevoke() {
    setActionLoading(true);
    try {
      const data = await post<{ affected: number }>("/admin/sessions/bulk-revoke", { ids: Array.from(selected) });
      notifications.show({ message: `${data.affected} session(s) revoked`, color: "green", icon: <IconCheck size={16} /> });
      setBulkRevokeOpen(false);
      setSelected(new Set());
      load();
    } catch {
      notifications.show({ message: "Failed to revoke sessions", color: "red" });
    } finally {
      setActionLoading(false);
    }
  }

  async function handleExport() {
    try {
      const params = new URLSearchParams({ sort: sort.column, order: sort.direction.toUpperCase() });
      await downloadCSV(`/admin/sessions/export?${params}`);
    } catch {
      notifications.show({ message: "Failed to export", color: "red" });
    }
  }

  const sessionIds = sessions.map((s) => s.id);
  const allSelected = sessionIds.length > 0 && sessionIds.every((id) => selected.has(id));

  const tableKeyboard = useTableKeyboard({
    rowCount: sessions.length,
    onSelect: (i) => { const s = sessions[i]; if (s) toggleSelect(s.id); },
  });

  return (
    <Stack>
      <Group justify="space-between" wrap="wrap">
        <Group gap="xs">
          <Title order={3}>Active Sessions</Title>
          {!loading && <Badge variant="light" size="lg">{sessions.length}</Badge>}
        </Group>
        <Button variant="subtle" size="xs" leftSection={<IconDownload size={16} />} onClick={handleExport}>
          Export CSV
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
                <Table.Th w={40}>
                  <Checkbox
                    checked={allSelected}
                    indeterminate={selected.size > 0 && !allSelected}
                    onChange={toggleAll}
                    aria-label="Select all"
                  />
                </Table.Th>
                <Table.Th>Token ID</Table.Th>
                <Table.Th style={{ cursor: "pointer" }} onClick={() => toggleSort("entity_type")}>
                  <Group gap={4} wrap="nowrap">Type <SortIcon column="entity_type" sort={sort} /></Group>
                </Table.Th>
                <Table.Th>Account</Table.Th>
                <Table.Th style={{ cursor: "pointer" }} onClick={() => toggleSort("created_at")}>
                  <Group gap={4} wrap="nowrap">Created <SortIcon column="created_at" sort={sort} /></Group>
                </Table.Th>
                <Table.Th style={{ cursor: "pointer" }} onClick={() => toggleSort("expires_at")}>
                  <Group gap={4} wrap="nowrap">Expires <SortIcon column="expires_at" sort={sort} /></Group>
                </Table.Th>
                <Table.Th ta="right">Actions</Table.Th>
              </Table.Tr>
            </Table.Thead>
            <Table.Tbody {...tableKeyboard.tbodyProps}>
              {sessions.length === 0 ? (
                <Table.Tr>
                  <Table.Td colSpan={7}>
                    <Text ta="center" c="dimmed" py="lg">No active sessions</Text>
                  </Table.Td>
                </Table.Tr>
              ) : (
                sessions.map((session, idx) => (
                  <Table.Tr
                    key={session.id}
                    bg={selected.has(session.id) ? "var(--mantine-primary-color-light)" : undefined}
                    style={tableKeyboard.isFocused(idx) ? { outline: "2px solid var(--mantine-primary-color-filled)", outlineOffset: -2 } : undefined}
                  >
                    <Table.Td>
                      <Checkbox
                        checked={selected.has(session.id)}
                        onChange={() => toggleSelect(session.id)}
                        aria-label={`Select session ${session.id.slice(0, 8)}`}
                      />
                    </Table.Td>
                    <Table.Td>
                      <Tooltip label={session.id}>
                        <Text size="sm" ff="monospace">{session.id.slice(0, 8)}...</Text>
                      </Tooltip>
                    </Table.Td>
                    <Table.Td>
                      <Badge variant="light" color={session.entity_type === "admin" ? "violet" : "blue"} size="sm">
                        {session.entity_type}
                      </Badge>
                    </Table.Td>
                    <Table.Td>
                      <Text size="sm">{session.email || session.name || `#${session.entity_id}`}</Text>
                    </Table.Td>
                    <Table.Td>
                      <Tooltip label={new Date(session.created_at).toLocaleString()}>
                        <Text size="sm" c="dimmed">{timeAgo(session.created_at)}</Text>
                      </Tooltip>
                    </Table.Td>
                    <Table.Td>
                      <Text size="sm" c="dimmed">{new Date(session.expires_at).toLocaleString()}</Text>
                    </Table.Td>
                    <Table.Td>
                      <Group justify="flex-end">
                        <Tooltip label="Revoke">
                          <ActionIcon variant="subtle" size="sm" color="red" onClick={() => setRevokeTarget(session)}>
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
      )}

      {selected.size > 0 && (
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
            <Text size="sm" fw={500}>{selected.size} selected</Text>
            <Button variant="light" color="red" size="xs" leftSection={<IconTrash size={14} />} onClick={() => setBulkRevokeOpen(true)}>
              Revoke
            </Button>
            <ActionIcon variant="subtle" size="sm" onClick={() => setSelected(new Set())}><IconX size={14} /></ActionIcon>
          </Group>
        </Box>
      )}

      {/* Revoke confirmation */}
      <Modal opened={!!revokeTarget} onClose={() => setRevokeTarget(null)} title="Revoke Session" size="sm">
        <Stack>
          <Text size="sm">Revoke session <strong>{revokeTarget?.id.slice(0, 8)}...</strong> for {revokeTarget?.email || revokeTarget?.name}?</Text>
          <Group justify="flex-end">
            <Button variant="default" onClick={() => setRevokeTarget(null)}>Cancel</Button>
            <Button color="red" onClick={handleRevoke} loading={actionLoading}>Revoke</Button>
          </Group>
        </Stack>
      </Modal>

      {/* Bulk revoke confirmation */}
      <Modal opened={bulkRevokeOpen} onClose={() => setBulkRevokeOpen(false)} title="Revoke Sessions" size="sm">
        <Stack>
          <Text size="sm">Revoke <strong>{selected.size}</strong> session(s)? Users will be logged out.</Text>
          <Group justify="flex-end">
            <Button variant="default" onClick={() => setBulkRevokeOpen(false)}>Cancel</Button>
            <Button color="red" onClick={handleBulkRevoke} loading={actionLoading}>Revoke {selected.size} session(s)</Button>
          </Group>
        </Stack>
      </Modal>
    </Stack>
  );
}
