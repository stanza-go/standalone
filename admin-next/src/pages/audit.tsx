import { useCallback, useEffect, useState } from "react";
import {
  ActionIcon,
  Alert,
  Badge,
  Box,
  Button,
  Group,
  Loader,
  NativeSelect,
  Pagination,
  SimpleGrid,
  Stack,
  Table,
  Text,
  TextInput,
  Title,
  Tooltip,
} from "@mantine/core";
import { DateInput } from "@mantine/dates";
import { notifications } from "@mantine/notifications";
import {
  IconAlertCircle,
  IconArrowDown,
  IconArrowUp,
  IconArrowsSort,
  IconChevronDown,
  IconChevronUp,
  IconDownload,
  IconFilter,
  IconSearch,
  IconX,
} from "@tabler/icons-react";
import { get, downloadCSV } from "@/lib/api";
import { useDebounce } from "@/hooks/use-debounce";
import { useSort } from "@/hooks/use-sort";

interface AuditEntry {
  id: number;
  admin_id: string;
  admin_email: string;
  admin_name: string;
  action: string;
  entity_type: string;
  entity_id: string;
  details: string;
  ip_address: string;
  created_at: string;
}

const PAGE_SIZE = 30;

const ACTION_LABELS: Record<string, string> = {
  "admin.create": "Created admin",
  "admin.update": "Updated admin",
  "admin.delete": "Deleted admin",
  "user.create": "Created user",
  "user.update": "Updated user",
  "user.delete": "Deleted user",
  "user.impersonate": "Impersonated user",
  "session.revoke": "Revoked session",
  "setting.update": "Updated setting",
  "api_key.create": "Created API key",
  "api_key.update": "Updated API key",
  "api_key.revoke": "Revoked API key",
  "cron.trigger": "Triggered cron job",
  "cron.enable": "Enabled cron job",
  "cron.disable": "Disabled cron job",
  "job.retry": "Retried job",
  "job.cancel": "Cancelled job",
};

const ACTION_COLORS: Record<string, string> = {
  create: "green",
  update: "blue",
  delete: "red",
  revoke: "orange",
  impersonate: "yellow",
  trigger: "violet",
  enable: "green",
  disable: "red",
  retry: "blue",
  cancel: "orange",
};

function actionColor(action: string): string {
  const verb = action.split(".")[1] ?? "";
  return ACTION_COLORS[verb] ?? "gray";
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

export default function AuditPage() {
  const [entries, setEntries] = useState<AuditEntry[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  // Filters
  const [searchInput, setSearchInput] = useState("");
  const search = useDebounce(searchInput, 300);
  const [actionFilter, setActionFilter] = useState("");
  const [dateFrom, setDateFrom] = useState<Date | null>(null);
  const [dateTo, setDateTo] = useState<Date | null>(null);

  const [sort, toggleSort] = useSort("id", "desc");
  const [expanded, setExpanded] = useState<Set<number>>(new Set());

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
      if (actionFilter) params.set("action", actionFilter);
      if (dateFrom) params.set("from", dateFrom.toISOString().split("T")[0] + "T00:00:00Z");
      if (dateTo) params.set("to", dateTo.toISOString().split("T")[0] + "T23:59:59Z");

      const data = await get<{ entries: AuditEntry[]; total: number }>(`/admin/audit?${params}`);
      setEntries(data.entries ?? []);
      setTotal(data.total);
    } catch {
      setError("Failed to load audit log");
    } finally {
      setLoading(false);
    }
  }, [page, search, actionFilter, dateFrom, dateTo, sort]);

  useEffect(() => {
    load();
  }, [load]);

  useEffect(() => {
    setPage(1);
  }, [search, actionFilter, dateFrom, dateTo]);

  function toggleExpand(id: number) {
    setExpanded((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  }

  async function handleExport() {
    try {
      const params = new URLSearchParams({
        sort: sort.column,
        order: sort.direction.toUpperCase(),
      });
      if (search) params.set("search", search);
      if (actionFilter) params.set("action", actionFilter);
      if (dateFrom) params.set("from", dateFrom.toISOString().split("T")[0] + "T00:00:00Z");
      if (dateTo) params.set("to", dateTo.toISOString().split("T")[0] + "T23:59:59Z");
      await downloadCSV(`/admin/audit/export?${params}`);
    } catch {
      notifications.show({ message: "Failed to export", color: "red" });
    }
  }

  const hasFilters = !!search || !!actionFilter || dateFrom !== null || dateTo !== null;
  const totalPages = Math.ceil(total / PAGE_SIZE);

  return (
    <Stack>
      {/* Header */}
      <Group justify="space-between">
        <Group gap="xs">
          <Title order={3}>Audit Log</Title>
          {!loading && <Badge variant="light" size="lg">{total}</Badge>}
        </Group>
        <Button variant="subtle" size="xs" leftSection={<IconDownload size={16} />} onClick={handleExport}>
          Export CSV
        </Button>
      </Group>

      {/* Filters */}
      <Group gap="sm" wrap="wrap">
        <TextInput
          placeholder="Search actions or details..."
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
          style={{ flex: 1, minWidth: 200, maxWidth: 350 }}
        />
        <NativeSelect
          leftSection={<IconFilter size={16} />}
          value={actionFilter}
          onChange={(e) => setActionFilter(e.currentTarget.value)}
          data={[
            { value: "", label: "All actions" },
            ...Object.entries(ACTION_LABELS).map(([key, label]) => ({ value: key, label })),
          ]}
          w={200}
        />
        <DateInput
          placeholder="From"
          value={dateFrom}
          onChange={setDateFrom}
          clearable
          w={150}
          size="sm"
        />
        <DateInput
          placeholder="To"
          value={dateTo}
          onChange={setDateTo}
          clearable
          w={150}
          size="sm"
        />
        {hasFilters && (
          <Button
            variant="subtle"
            size="xs"
            leftSection={<IconX size={14} />}
            onClick={() => {
              setSearchInput("");
              setActionFilter("");
              setDateFrom(null);
              setDateTo(null);
            }}
          >
            Clear
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
      {loading && entries.length === 0 ? (
        <Group justify="center" pt="xl">
          <Loader />
        </Group>
      ) : (
        <>
          <Table.ScrollContainer minWidth={600}>
            <Table>
              <Table.Thead>
                <Table.Tr>
                  <Table.Th w={40}></Table.Th>
                  <Table.Th style={{ cursor: "pointer" }} onClick={() => toggleSort("created_at")}>
                    <Group gap={4} wrap="nowrap">Time <SortIcon column="created_at" sort={sort} /></Group>
                  </Table.Th>
                  <Table.Th>Admin</Table.Th>
                  <Table.Th style={{ cursor: "pointer" }} onClick={() => toggleSort("action")}>
                    <Group gap={4} wrap="nowrap">Action <SortIcon column="action" sort={sort} /></Group>
                  </Table.Th>
                  <Table.Th style={{ cursor: "pointer" }} onClick={() => toggleSort("entity_type")}>
                    <Group gap={4} wrap="nowrap">Target <SortIcon column="entity_type" sort={sort} /></Group>
                  </Table.Th>
                  <Table.Th>Details</Table.Th>
                  <Table.Th>IP</Table.Th>
                </Table.Tr>
              </Table.Thead>
              <Table.Tbody>
                {entries.length === 0 ? (
                  <Table.Tr>
                    <Table.Td colSpan={7}>
                      <Text ta="center" c="dimmed" py="lg">
                        {hasFilters ? "No events match your filters" : "No audit events recorded yet"}
                      </Text>
                    </Table.Td>
                  </Table.Tr>
                ) : (
                  entries.map((entry) => {
                    const isExpanded = expanded.has(entry.id);
                    return (
                      <Table.Tr
                        key={entry.id}
                        style={{ cursor: "pointer" }}
                        onClick={() => toggleExpand(entry.id)}
                      >
                        <Table.Td>
                          {isExpanded ? <IconChevronUp size={16} /> : <IconChevronDown size={16} />}
                        </Table.Td>
                        <Table.Td>
                          <Tooltip label={new Date(entry.created_at).toLocaleString()}>
                            <Text size="sm" c="dimmed">{timeAgo(entry.created_at)}</Text>
                          </Tooltip>
                        </Table.Td>
                        <Table.Td>
                          <Text size="sm" fw={500}>{entry.admin_name || entry.admin_email || `Admin #${entry.admin_id}`}</Text>
                          {entry.admin_name && entry.admin_email && (
                            <Text size="xs" c="dimmed">{entry.admin_email}</Text>
                          )}
                        </Table.Td>
                        <Table.Td>
                          <Badge variant="light" color={actionColor(entry.action)} size="sm">
                            {ACTION_LABELS[entry.action] || entry.action}
                          </Badge>
                        </Table.Td>
                        <Table.Td>
                          {entry.entity_type && (
                            <Text size="xs" ff="monospace" c="dimmed">
                              {entry.entity_type}{entry.entity_id ? `#${entry.entity_id}` : ""}
                            </Text>
                          )}
                        </Table.Td>
                        <Table.Td>
                          <Text size="xs" c="dimmed" truncate maw={200}>{entry.details || "\u2014"}</Text>
                        </Table.Td>
                        <Table.Td>
                          <Text size="xs" ff="monospace" c="dimmed">{entry.ip_address}</Text>
                        </Table.Td>
                      </Table.Tr>
                    );
                  })
                )}
              </Table.Tbody>
            </Table>
          </Table.ScrollContainer>

          {/* Expanded detail panels */}
          {entries.filter((e) => expanded.has(e.id)).map((entry) => (
            <Box key={`detail-${entry.id}`} px="md" py="xs" bg="var(--mantine-color-default-hover)">
              <SimpleGrid cols={{ base: 2, sm: 4 }} spacing="xs">
                <div>
                  <Text size="xs" c="dimmed">Event ID</Text>
                  <Text size="xs" ff="monospace">{entry.id}</Text>
                </div>
                <div>
                  <Text size="xs" c="dimmed">Admin ID</Text>
                  <Text size="xs" ff="monospace">{entry.admin_id}</Text>
                </div>
                <div>
                  <Text size="xs" c="dimmed">Action</Text>
                  <Text size="xs" ff="monospace">{entry.action}</Text>
                </div>
                <div>
                  <Text size="xs" c="dimmed">Time</Text>
                  <Text size="xs">{new Date(entry.created_at).toLocaleString()}</Text>
                </div>
                <div>
                  <Text size="xs" c="dimmed">Target</Text>
                  <Text size="xs" ff="monospace">{entry.entity_type}{entry.entity_id ? `#${entry.entity_id}` : ""}</Text>
                </div>
                <div>
                  <Text size="xs" c="dimmed">IP Address</Text>
                  <Text size="xs" ff="monospace">{entry.ip_address}</Text>
                </div>
                {entry.details && (
                  <div style={{ gridColumn: "span 2" }}>
                    <Text size="xs" c="dimmed">Details</Text>
                    <Text size="xs">{entry.details}</Text>
                  </div>
                )}
              </SimpleGrid>
            </Box>
          ))}

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
    </Stack>
  );
}
