import { useCallback, useEffect, useState } from "react";
import {
  ActionIcon,
  Alert,
  Badge,
  Box,
  Button,
  Checkbox,
  Code,
  Group,
  Loader,
  Modal,
  NativeSelect,
  Pagination,
  Paper,
  SimpleGrid,
  Stack,
  Table,
  Text,
  TextInput,
  Title,
  Tooltip,
} from "@mantine/core";
import { notifications } from "@mantine/notifications";
import {
  IconAlertCircle,
  IconBan,
  IconCheck,
  IconChevronDown,
  IconChevronRight,
  IconCircleCheck,
  IconCircleX,
  IconInbox,
  IconPlayerPlay,
  IconRefresh,
  IconRotateClockwise,
  IconSearch,
  IconSkull,
  IconTrash,
  IconX,
} from "@tabler/icons-react";
import { get, post } from "@/lib/api";
import { useSelection } from "@/hooks/use-selection";
import { useTableKeyboard } from "@/hooks/use-table-keyboard";

interface QueueStats {
  pending: number;
  running: number;
  completed: number;
  failed: number;
  dead: number;
  cancelled: number;
}

interface QueueJob {
  id: number;
  queue: string;
  type: string;
  payload: string;
  status: string;
  attempts: number;
  max_attempts: number;
  last_error: string;
  run_at: string;
  started_at: string;
  completed_at: string;
  created_at: string;
  updated_at: string;
}

const PAGE_SIZE = 25;
const STATUS_OPTIONS = [
  { value: "", label: "All statuses" },
  { value: "pending", label: "Pending" },
  { value: "running", label: "Running" },
  { value: "completed", label: "Completed" },
  { value: "failed", label: "Failed" },
  { value: "dead", label: "Dead" },
  { value: "cancelled", label: "Cancelled" },
];

function formatTime(iso: string): string {
  if (!iso) return "\u2014";
  return new Date(iso).toLocaleString();
}

function formatDuration(startIso: string, endIso: string): string {
  if (!startIso || !endIso) return "\u2014";
  const ms = new Date(endIso).getTime() - new Date(startIso).getTime();
  if (ms < 1000) return `${ms}ms`;
  const secs = ms / 1000;
  if (secs < 60) return `${secs.toFixed(1)}s`;
  const mins = Math.floor(secs / 60);
  const remSecs = Math.round(secs % 60);
  return `${mins}m ${remSecs}s`;
}

function formatPayload(raw: string): string {
  try {
    return JSON.stringify(JSON.parse(raw), null, 2);
  } catch {
    return raw;
  }
}

const statusColors: Record<string, string> = {
  pending: "yellow",
  running: "blue",
  completed: "green",
  failed: "red",
  dead: "gray",
  cancelled: "gray",
};

const statusIcons: Record<string, React.ReactNode> = {
  pending: <IconInbox size={10} />,
  running: <IconPlayerPlay size={10} />,
  completed: <IconCircleCheck size={10} />,
  failed: <IconCircleX size={10} />,
  dead: <IconSkull size={10} />,
  cancelled: <IconBan size={10} />,
};

function StatCard({ label, value, icon }: { label: string; value: number; icon: React.ReactNode }) {
  return (
    <Paper withBorder p="md" radius="md">
      <Group justify="space-between">
        <div>
          <Text size="xs" c="dimmed" tt="uppercase" fw={600}>{label}</Text>
          <Text size="xl" fw={700} mt={4}>{value}</Text>
        </div>
        <Box c="dimmed">{icon}</Box>
      </Group>
    </Paper>
  );
}

function JobDetail({ job }: { job: QueueJob }) {
  return (
    <Box px="md" py="sm">
      <SimpleGrid cols={{ base: 1, sm: 2 }} spacing="md">
        <div>
          <Text size="xs" fw={600} tt="uppercase" c="dimmed" mb="xs">Details</Text>
          <Stack gap={4}>
            <Group gap="sm"><Text size="sm" c="dimmed" w={90}>ID</Text><Text size="sm" ff="monospace">{job.id}</Text></Group>
            <Group gap="sm"><Text size="sm" c="dimmed" w={90}>Queue</Text><Text size="sm" ff="monospace">{job.queue}</Text></Group>
            <Group gap="sm"><Text size="sm" c="dimmed" w={90}>Type</Text><Text size="sm" ff="monospace">{job.type}</Text></Group>
            <Group gap="sm"><Text size="sm" c="dimmed" w={90}>Status</Text><Badge variant="light" color={statusColors[job.status] || "gray"} size="sm" leftSection={statusIcons[job.status]}>{job.status}</Badge></Group>
            <Group gap="sm"><Text size="sm" c="dimmed" w={90}>Attempts</Text><Text size="sm">{job.attempts}/{job.max_attempts}</Text></Group>
          </Stack>
        </div>
        <div>
          <Text size="xs" fw={600} tt="uppercase" c="dimmed" mb="xs">Timing</Text>
          <Stack gap={4}>
            <Group gap="sm"><Text size="sm" c="dimmed" w={90}>Created</Text><Text size="sm">{formatTime(job.created_at)}</Text></Group>
            <Group gap="sm"><Text size="sm" c="dimmed" w={90}>Run at</Text><Text size="sm">{formatTime(job.run_at)}</Text></Group>
            <Group gap="sm"><Text size="sm" c="dimmed" w={90}>Started</Text><Text size="sm">{formatTime(job.started_at)}</Text></Group>
            <Group gap="sm"><Text size="sm" c="dimmed" w={90}>Completed</Text><Text size="sm">{formatTime(job.completed_at)}</Text></Group>
            <Group gap="sm"><Text size="sm" c="dimmed" w={90}>Duration</Text><Text size="sm">{formatDuration(job.started_at, job.completed_at)}</Text></Group>
          </Stack>
        </div>
      </SimpleGrid>

      {job.payload && job.payload !== "{}" && (
        <Box mt="md">
          <Text size="xs" fw={600} tt="uppercase" c="dimmed" mb="xs">Payload</Text>
          <Code block style={{ maxHeight: 200, overflow: "auto" }}>{formatPayload(job.payload)}</Code>
        </Box>
      )}

      {job.last_error && (
        <Box mt="md">
          <Text size="xs" fw={600} tt="uppercase" c="dimmed" mb="xs">Last Error</Text>
          <Code block color="red" style={{ maxHeight: 120, overflow: "auto" }}>{job.last_error}</Code>
        </Box>
      )}
    </Box>
  );
}

export default function QueuePage() {
  const [stats, setStats] = useState<QueueStats | null>(null);
  const [jobs, setJobs] = useState<QueueJob[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [statusFilter, setStatusFilter] = useState("");
  const [typeFilter, setTypeFilter] = useState("");
  const [queueFilter, setQueueFilter] = useState("");
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [acting, setActing] = useState<number | null>(null);
  const [expanded, setExpanded] = useState<number | null>(null);
  const selection = useSelection();

  // Modal states
  const [cancelTarget, setCancelTarget] = useState<QueueJob | null>(null);
  const [bulkRetryOpen, setBulkRetryOpen] = useState(false);
  const [bulkCancelOpen, setBulkCancelOpen] = useState(false);
  const [actionLoading, setActionLoading] = useState(false);

  const loadStats = useCallback(async () => {
    try {
      const data = await get<QueueStats>("/admin/queue/stats");
      setStats(data);
    } catch {
      // Stats are supplementary — don't block main view.
    }
  }, []);

  const loadJobs = useCallback(async () => {
    try {
      const offset = (page - 1) * PAGE_SIZE;
      const params = new URLSearchParams({ limit: String(PAGE_SIZE), offset: String(offset) });
      if (statusFilter) params.set("status", statusFilter);
      if (typeFilter) params.set("type", typeFilter);
      if (queueFilter) params.set("queue", queueFilter);
      const data = await get<{ jobs: QueueJob[]; total: number }>(`/admin/queue/jobs?${params}`);
      setJobs(data.jobs);
      setTotal(data.total);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load jobs");
    }
  }, [page, statusFilter, typeFilter, queueFilter]);

  const loadAll = useCallback(async () => {
    setLoading(true);
    await Promise.all([loadStats(), loadJobs()]);
    setLoading(false);
  }, [loadStats, loadJobs]);

  useEffect(() => {
    loadAll();
    const interval = setInterval(loadAll, 10_000);
    return () => clearInterval(interval);
  }, [loadAll]);

  useEffect(() => {
    selection.clear();
  }, [statusFilter, typeFilter, queueFilter, page]); // eslint-disable-line react-hooks/exhaustive-deps

  function resetFilters() {
    setStatusFilter("");
    setTypeFilter("");
    setQueueFilter("");
    setPage(1);
    setExpanded(null);
  }

  async function retryJob(id: number) {
    setActing(id);
    try {
      await post(`/admin/queue/jobs/${id}/retry`);
      notifications.show({ message: "Job queued for retry", color: "green", icon: <IconCheck size={16} /> });
      await loadAll();
    } catch (err) {
      notifications.show({ message: err instanceof Error ? err.message : "Failed to retry job", color: "red" });
    } finally {
      setActing(null);
    }
  }

  async function cancelJob() {
    if (!cancelTarget) return;
    setActionLoading(true);
    try {
      await post(`/admin/queue/jobs/${cancelTarget.id}/cancel`);
      notifications.show({ message: "Job cancelled", color: "green", icon: <IconCheck size={16} /> });
      setCancelTarget(null);
      await loadAll();
    } catch (err) {
      notifications.show({ message: err instanceof Error ? err.message : "Failed to cancel job", color: "red" });
    } finally {
      setActionLoading(false);
    }
  }

  async function handleBulkRetry() {
    setActionLoading(true);
    try {
      const data = await post<{ affected: number }>("/admin/queue/jobs/bulk-retry", { ids: selection.ids });
      notifications.show({ message: `${data.affected} job(s) retried`, color: "green", icon: <IconCheck size={16} /> });
      setBulkRetryOpen(false);
      selection.clear();
      await loadAll();
    } catch (err) {
      notifications.show({ message: err instanceof Error ? err.message : "Bulk retry failed", color: "red" });
    } finally {
      setActionLoading(false);
    }
  }

  async function handleBulkCancel() {
    setActionLoading(true);
    try {
      const data = await post<{ affected: number }>("/admin/queue/jobs/bulk-cancel", { ids: selection.ids });
      notifications.show({ message: `${data.affected} job(s) cancelled`, color: "green", icon: <IconCheck size={16} /> });
      setBulkCancelOpen(false);
      selection.clear();
      await loadAll();
    } catch (err) {
      notifications.show({ message: err instanceof Error ? err.message : "Bulk cancel failed", color: "red" });
    } finally {
      setActionLoading(false);
    }
  }

  const totalPages = Math.ceil(total / PAGE_SIZE);
  const jobIds = jobs.map((j) => j.id);
  const hasActiveFilters = typeFilter !== "" || queueFilter !== "";

  const tableKeyboard = useTableKeyboard({
    rowCount: jobs.length,
    onActivate: (i) => { const j = jobs[i]; if (j) setExpanded((prev) => (prev === j.id ? null : j.id)); },
    onSelect: (i) => { const j = jobs[i]; if (j) selection.toggle(j.id); },
  });

  const statCards = stats
    ? [
        { label: "Pending", value: stats.pending, icon: <IconInbox size={20} /> },
        { label: "Running", value: stats.running, icon: <IconPlayerPlay size={20} /> },
        { label: "Completed", value: stats.completed, icon: <IconCircleCheck size={20} /> },
        { label: "Failed", value: stats.failed, icon: <IconCircleX size={20} /> },
        { label: "Dead", value: stats.dead, icon: <IconSkull size={20} /> },
        { label: "Cancelled", value: stats.cancelled, icon: <IconBan size={20} /> },
      ]
    : [];

  return (
    <Stack>
      {/* Header */}
      <Group justify="space-between">
        <Title order={3}>Job Queue</Title>
        <Button
          variant="subtle"
          size="xs"
          leftSection={<IconRefresh size={16} />}
          onClick={loadAll}
          loading={loading}
        >
          Refresh
        </Button>
      </Group>

      {/* Error */}
      {error && (
        <Alert icon={<IconAlertCircle size={16} />} color="red" variant="light" withCloseButton onClose={() => setError("")}>
          {error}
          <Button variant="subtle" size="xs" ml="sm" onClick={loadAll}>Retry</Button>
        </Alert>
      )}

      {/* Stats cards */}
      {stats && (
        <SimpleGrid cols={{ base: 2, sm: 3, lg: 6 }}>
          {statCards.map((c) => (
            <StatCard key={c.label} label={c.label} value={c.value} icon={c.icon} />
          ))}
        </SimpleGrid>
      )}

      {/* Filters */}
      <Group gap="sm">
        <NativeSelect
          value={statusFilter}
          onChange={(e) => { setStatusFilter(e.currentTarget.value); setPage(1); setExpanded(null); }}
          data={STATUS_OPTIONS}
          size="xs"
          w={160}
        />
        <TextInput
          placeholder="Filter by type..."
          size="xs"
          leftSection={<IconSearch size={14} />}
          value={typeFilter}
          onChange={(e) => { setTypeFilter(e.currentTarget.value); setPage(1); setExpanded(null); }}
          rightSection={
            typeFilter ? (
              <ActionIcon variant="subtle" size="xs" onClick={() => { setTypeFilter(""); setPage(1); }}>
                <IconX size={12} />
              </ActionIcon>
            ) : null
          }
          w={160}
        />
        <TextInput
          placeholder="Filter by queue..."
          size="xs"
          leftSection={<IconSearch size={14} />}
          value={queueFilter}
          onChange={(e) => { setQueueFilter(e.currentTarget.value); setPage(1); setExpanded(null); }}
          rightSection={
            queueFilter ? (
              <ActionIcon variant="subtle" size="xs" onClick={() => { setQueueFilter(""); setPage(1); }}>
                <IconX size={12} />
              </ActionIcon>
            ) : null
          }
          w={160}
        />
        {hasActiveFilters && (
          <Button variant="subtle" size="xs" onClick={resetFilters}>Clear filters</Button>
        )}
      </Group>

      {/* Table */}
      {loading && jobs.length === 0 ? (
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
                      checked={selection.isAllSelected(jobIds)}
                      indeterminate={selection.count > 0 && !selection.isAllSelected(jobIds)}
                      onChange={() => selection.toggleAll(jobIds)}
                      aria-label="Select all"
                    />
                  </Table.Th>
                  <Table.Th w={32}></Table.Th>
                  <Table.Th>Type</Table.Th>
                  <Table.Th>Status</Table.Th>
                  <Table.Th visibleFrom="md">Attempts</Table.Th>
                  <Table.Th visibleFrom="md">Created</Table.Th>
                  <Table.Th visibleFrom="lg">Error</Table.Th>
                  <Table.Th ta="right">Actions</Table.Th>
                </Table.Tr>
              </Table.Thead>
              <Table.Tbody {...tableKeyboard.tbodyProps}>
                {jobs.length === 0 ? (
                  <Table.Tr>
                    <Table.Td colSpan={8}>
                      <Text ta="center" c="dimmed" py="lg">
                        {statusFilter || typeFilter || queueFilter ? "No jobs match your filters" : "No jobs found"}
                      </Text>
                    </Table.Td>
                  </Table.Tr>
                ) : (
                  jobs.map((job, idx) => (
                    <Box key={job.id} component="tbody">
                      <Table.Tr
                        bg={selection.isSelected(job.id) ? "var(--mantine-primary-color-light)" : undefined}
                        style={{ cursor: "pointer", ...(tableKeyboard.isFocused(idx) ? { outline: "2px solid var(--mantine-primary-color-filled)", outlineOffset: -2 } : {}) }}
                        onClick={() => setExpanded((prev) => (prev === job.id ? null : job.id))}
                      >
                        <Table.Td onClick={(e) => e.stopPropagation()}>
                          <Checkbox
                            checked={selection.isSelected(job.id)}
                            onChange={() => selection.toggle(job.id)}
                            aria-label={`Select job ${job.id}`}
                          />
                        </Table.Td>
                        <Table.Td>
                          {expanded === job.id ? (
                            <IconChevronDown size={14} style={{ opacity: 0.5 }} />
                          ) : (
                            <IconChevronRight size={14} style={{ opacity: 0.5 }} />
                          )}
                        </Table.Td>
                        <Table.Td>
                          <Text size="xs" ff="monospace">{job.type}</Text>
                        </Table.Td>
                        <Table.Td>
                          <Badge variant="light" color={statusColors[job.status] || "gray"} size="sm" leftSection={statusIcons[job.status]}>
                            {job.status}
                          </Badge>
                        </Table.Td>
                        <Table.Td visibleFrom="md">
                          <Text size="sm" c="dimmed">{job.attempts}/{job.max_attempts}</Text>
                        </Table.Td>
                        <Table.Td visibleFrom="md">
                          <Text size="sm" c="dimmed">{formatTime(job.created_at)}</Text>
                        </Table.Td>
                        <Table.Td visibleFrom="lg">
                          <Text size="xs" c="red" truncate maw={200}>{job.last_error || "\u2014"}</Text>
                        </Table.Td>
                        <Table.Td onClick={(e) => e.stopPropagation()}>
                          <Group gap={4} justify="flex-end" wrap="nowrap">
                            {(job.status === "failed" || job.status === "dead") && (
                              <Tooltip label="Retry">
                                <ActionIcon
                                  variant="subtle"
                                  size="sm"
                                  onClick={() => retryJob(job.id)}
                                  loading={acting === job.id}
                                  disabled={acting !== null}
                                >
                                  <IconRotateClockwise size={16} />
                                </ActionIcon>
                              </Tooltip>
                            )}
                            {job.status === "pending" && (
                              <Tooltip label="Cancel">
                                <ActionIcon
                                  variant="subtle"
                                  size="sm"
                                  color="red"
                                  onClick={() => setCancelTarget(job)}
                                  disabled={acting !== null}
                                >
                                  <IconBan size={16} />
                                </ActionIcon>
                              </Tooltip>
                            )}
                          </Group>
                        </Table.Td>
                      </Table.Tr>
                      {expanded === job.id && (
                        <Table.Tr>
                          <Table.Td colSpan={8} p={0} bg="var(--mantine-color-gray-light)">
                            <JobDetail job={job} />
                          </Table.Td>
                        </Table.Tr>
                      )}
                    </Box>
                  ))
                )}
              </Table.Tbody>
            </Table>
          </Table.ScrollContainer>

          {/* Pagination */}
          {totalPages > 1 && (
            <Group justify="space-between">
              <Text size="sm" c="dimmed">
                {(page - 1) * PAGE_SIZE + 1}–{Math.min(page * PAGE_SIZE, total)} of {total}
              </Text>
              <Pagination value={page} onChange={(p) => { setPage(p); setExpanded(null); }} total={totalPages} size="sm" />
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
              size="xs"
              leftSection={<IconRotateClockwise size={14} />}
              onClick={() => setBulkRetryOpen(true)}
            >
              Retry
            </Button>
            <Button
              variant="light"
              color="red"
              size="xs"
              leftSection={<IconTrash size={14} />}
              onClick={() => setBulkCancelOpen(true)}
            >
              Cancel
            </Button>
            <ActionIcon variant="subtle" size="sm" onClick={selection.clear}>
              <IconX size={14} />
            </ActionIcon>
          </Group>
        </Box>
      )}

      {/* Cancel confirmation */}
      <Modal opened={!!cancelTarget} onClose={() => setCancelTarget(null)} title="Cancel Job">
        <Stack>
          <Text size="sm">Are you sure you want to cancel this job? It will not be executed.</Text>
          {cancelTarget && (
            <Box>
              <Text size="sm"><strong>Type:</strong> <Text span ff="monospace" size="xs">{cancelTarget.type}</Text></Text>
              <Text size="sm"><strong>Queue:</strong> <Text span ff="monospace" size="xs">{cancelTarget.queue}</Text></Text>
            </Box>
          )}
          <Group justify="flex-end">
            <Button variant="default" onClick={() => setCancelTarget(null)}>Cancel</Button>
            <Button color="red" onClick={cancelJob} loading={actionLoading}>Cancel Job</Button>
          </Group>
        </Stack>
      </Modal>

      {/* Bulk retry confirmation */}
      <Modal opened={bulkRetryOpen} onClose={() => setBulkRetryOpen(false)} title="Retry Jobs">
        <Stack>
          <Text size="sm">
            Are you sure you want to retry <strong>{selection.count}</strong> job(s)?
          </Text>
          <Group justify="flex-end">
            <Button variant="default" onClick={() => setBulkRetryOpen(false)}>Cancel</Button>
            <Button onClick={handleBulkRetry} loading={actionLoading}>Retry</Button>
          </Group>
        </Stack>
      </Modal>

      {/* Bulk cancel confirmation */}
      <Modal opened={bulkCancelOpen} onClose={() => setBulkCancelOpen(false)} title="Cancel Jobs">
        <Stack>
          <Text size="sm">
            Are you sure you want to cancel <strong>{selection.count}</strong> job(s)? They will not be executed.
          </Text>
          <Group justify="flex-end">
            <Button variant="default" onClick={() => setBulkCancelOpen(false)}>Cancel</Button>
            <Button color="red" onClick={handleBulkCancel} loading={actionLoading}>Cancel Jobs</Button>
          </Group>
        </Stack>
      </Modal>
    </Stack>
  );
}
