import { useCallback, useEffect, useState } from "react";
import {
  ActionIcon,
  Alert,
  Badge,
  Box,
  Button,
  Collapse,
  Group,
  Loader,
  Pagination,
  Paper,
  SimpleGrid,
  Stack,
  Table,
  Text,
  Title,
  Tooltip,
} from "@mantine/core";
import { notifications } from "@mantine/notifications";
import {
  IconAlertCircle,
  IconCheck,
  IconChevronDown,
  IconChevronRight,
  IconClock,
  IconHistory,
  IconPlayerPause,
  IconPlayerPlay,
  IconRefresh,
  IconX,
} from "@tabler/icons-react";
import { get, post } from "@/lib/api";

interface CronEntry {
  name: string;
  schedule: string;
  enabled: boolean;
  running: boolean;
  last_run: string;
  next_run: string;
  last_err: string;
}

interface CronRun {
  id: number;
  name: string;
  started_at: string;
  duration_ms: number;
  status: string;
  error: string;
}

function relativeTime(iso: string): string {
  if (!iso) return "";
  const diff = new Date(iso).getTime() - Date.now();
  const abs = Math.abs(diff);
  if (abs < 60_000) return diff > 0 ? "in <1m" : "<1m ago";
  const mins = Math.round(abs / 60_000);
  if (mins < 60) return diff > 0 ? `in ${mins}m` : `${mins}m ago`;
  const hrs = Math.round(mins / 60);
  return diff > 0 ? `in ${hrs}h` : `${hrs}h ago`;
}

function formatDuration(ms: number): string {
  if (ms < 1000) return `${ms}ms`;
  const secs = ms / 1000;
  if (secs < 60) return `${secs.toFixed(1)}s`;
  const mins = Math.floor(secs / 60);
  const remSecs = Math.round(secs % 60);
  return `${mins}m ${remSecs}s`;
}

const RUNS_PAGE_SIZE = 20;

function RunHistory({ name }: { name: string }) {
  const [runs, setRuns] = useState<CronRun[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const offset = (page - 1) * RUNS_PAGE_SIZE;
      const data = await get<{ runs: CronRun[]; total: number }>(
        `/admin/cron/${encodeURIComponent(name)}/runs?limit=${RUNS_PAGE_SIZE}&offset=${offset}`,
      );
      setRuns(data.runs);
      setTotal(data.total);
      setError("");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load run history");
    } finally {
      setLoading(false);
    }
  }, [name, page]);

  useEffect(() => {
    load();
  }, [load]);

  if (loading) {
    return (
      <Group justify="center" py="md">
        <Loader size="sm" />
      </Group>
    );
  }

  if (error) {
    return (
      <Alert icon={<IconAlertCircle size={16} />} color="red" variant="light" mx="md" my="xs">
        {error}
        <Button variant="subtle" size="xs" ml="sm" onClick={load}>Retry</Button>
      </Alert>
    );
  }

  if (runs.length === 0) {
    return (
      <Text ta="center" c="dimmed" size="sm" py="md">
        No execution history yet
      </Text>
    );
  }

  const totalPages = Math.ceil(total / RUNS_PAGE_SIZE);

  return (
    <Stack gap="xs" px="md" pb="sm">
      <Table fz="xs">
        <Table.Thead>
          <Table.Tr>
            <Table.Th>Started</Table.Th>
            <Table.Th>Duration</Table.Th>
            <Table.Th>Status</Table.Th>
            <Table.Th visibleFrom="md">Error</Table.Th>
          </Table.Tr>
        </Table.Thead>
        <Table.Tbody>
          {runs.map((run) => (
            <Table.Tr key={run.id}>
              <Table.Td>
                <Tooltip label={new Date(run.started_at).toLocaleString()}>
                  <Text size="xs" c="dimmed">{relativeTime(run.started_at)}</Text>
                </Tooltip>
              </Table.Td>
              <Table.Td>
                <Text size="xs" ff="monospace">{formatDuration(run.duration_ms)}</Text>
              </Table.Td>
              <Table.Td>
                <Badge
                  variant="light"
                  color={run.status === "success" ? "green" : "red"}
                  size="xs"
                  leftSection={run.status === "success" ? <IconCheck size={10} /> : <IconX size={10} />}
                >
                  {run.status === "success" ? "Success" : "Error"}
                </Badge>
              </Table.Td>
              <Table.Td visibleFrom="md">
                <Text size="xs" c="red" truncate maw={300}>
                  {run.error || "\u2014"}
                </Text>
              </Table.Td>
            </Table.Tr>
          ))}
        </Table.Tbody>
      </Table>
      {totalPages > 1 && (
        <Group justify="space-between">
          <Text size="xs" c="dimmed">
            {(page - 1) * RUNS_PAGE_SIZE + 1}–{Math.min(page * RUNS_PAGE_SIZE, total)} of {total}
          </Text>
          <Pagination value={page} onChange={setPage} total={totalPages} size="xs" />
        </Group>
      )}
    </Stack>
  );
}

export default function CronPage() {
  const [entries, setEntries] = useState<CronEntry[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [acting, setActing] = useState<string | null>(null);
  const [expanded, setExpanded] = useState<string | null>(null);

  const load = useCallback(async () => {
    try {
      const data = await get<{ entries: CronEntry[] }>("/admin/cron");
      setEntries(data.entries);
      setError("");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load cron jobs");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    load();
    const interval = setInterval(load, 10_000);
    return () => clearInterval(interval);
  }, [load]);

  async function action(name: string, act: string) {
    setActing(`${name}:${act}`);
    try {
      await post(`/admin/cron/${encodeURIComponent(name)}/${act}`);
      const labels: Record<string, string> = { trigger: "triggered", enable: "enabled", disable: "disabled" };
      notifications.show({ message: `Cron job ${labels[act] || act}`, color: "green", icon: <IconCheck size={16} /> });
      await load();
    } catch (err) {
      notifications.show({ message: err instanceof Error ? err.message : `Failed to ${act} job`, color: "red" });
    } finally {
      setActing(null);
    }
  }

  function toggleExpand(name: string) {
    setExpanded((prev) => (prev === name ? null : name));
  }

  return (
    <Stack>
      {/* Header */}
      <Group justify="space-between">
        <Group gap="xs">
          <Title order={3}>Cron Jobs</Title>
          {!loading && <Badge variant="light" size="lg">{entries.length}</Badge>}
        </Group>
        <Button
          variant="subtle"
          size="xs"
          leftSection={<IconRefresh size={16} />}
          onClick={load}
          loading={loading}
        >
          Refresh
        </Button>
      </Group>

      {/* Error */}
      {error && (
        <Alert icon={<IconAlertCircle size={16} />} color="red" variant="light" withCloseButton onClose={() => setError("")}>
          {error}
          <Button variant="subtle" size="xs" ml="sm" onClick={load}>Retry</Button>
        </Alert>
      )}

      {/* Loading */}
      {loading && entries.length === 0 ? (
        <Group justify="center" pt="xl">
          <Loader />
        </Group>
      ) : entries.length === 0 && !error ? (
        <Text ta="center" c="dimmed" py="xl">No cron jobs registered</Text>
      ) : (
        /* Job cards */
        <Stack gap="sm">
          {entries.map((entry) => (
            <Paper key={entry.name} withBorder radius="md" p={0}>
              {/* Main row */}
              <Box
                px="md"
                py="sm"
                style={{ cursor: "pointer" }}
                onClick={() => toggleExpand(entry.name)}
              >
                <Group justify="space-between" wrap="nowrap">
                  <Group gap="sm" wrap="nowrap">
                    {expanded === entry.name ? (
                      <IconChevronDown size={16} style={{ opacity: 0.5, flexShrink: 0 }} />
                    ) : (
                      <IconChevronRight size={16} style={{ opacity: 0.5, flexShrink: 0 }} />
                    )}
                    <div>
                      <Text size="sm" fw={500} ff="monospace">{entry.name}</Text>
                      <Text size="xs" c="dimmed" ff="monospace" hiddenFrom="sm" mt={2}>{entry.schedule}</Text>
                    </div>
                  </Group>
                  <Group gap="xs" wrap="nowrap" onClick={(e) => e.stopPropagation()}>
                    <Tooltip label="Trigger now">
                      <ActionIcon
                        variant="subtle"
                        size="sm"
                        onClick={() => action(entry.name, "trigger")}
                        loading={acting === `${entry.name}:trigger`}
                        disabled={acting !== null}
                      >
                        <IconPlayerPlay size={16} />
                      </ActionIcon>
                    </Tooltip>
                    {entry.enabled ? (
                      <Tooltip label="Disable">
                        <ActionIcon
                          variant="subtle"
                          size="sm"
                          onClick={() => action(entry.name, "disable")}
                          loading={acting === `${entry.name}:disable`}
                          disabled={acting !== null}
                        >
                          <IconPlayerPause size={16} />
                        </ActionIcon>
                      </Tooltip>
                    ) : (
                      <Tooltip label="Enable">
                        <ActionIcon
                          variant="subtle"
                          size="sm"
                          onClick={() => action(entry.name, "enable")}
                          loading={acting === `${entry.name}:enable`}
                          disabled={acting !== null}
                        >
                          <IconClock size={16} />
                        </ActionIcon>
                      </Tooltip>
                    )}
                  </Group>
                </Group>

                {/* Details row */}
                <SimpleGrid cols={{ base: 2, sm: 4 }} mt="xs" spacing="xs">
                  <div>
                    <Text size="xs" c="dimmed">Schedule</Text>
                    <Text size="xs" ff="monospace" visibleFrom="sm">{entry.schedule}</Text>
                    <Text size="xs" ff="monospace" hiddenFrom="sm">{entry.schedule}</Text>
                  </div>
                  <div>
                    <Text size="xs" c="dimmed">Status</Text>
                    {entry.running ? (
                      <Badge variant="light" color="blue" size="xs" leftSection={<Loader size={8} />}>
                        Running
                      </Badge>
                    ) : entry.enabled ? (
                      <Badge variant="light" color="green" size="xs">Enabled</Badge>
                    ) : (
                      <Badge variant="light" color="gray" size="xs">Disabled</Badge>
                    )}
                  </div>
                  <div>
                    <Text size="xs" c="dimmed">Last Run</Text>
                    {entry.last_run ? (
                      <Tooltip label={new Date(entry.last_run).toLocaleString()}>
                        <Text size="xs">{relativeTime(entry.last_run)}</Text>
                      </Tooltip>
                    ) : (
                      <Text size="xs" c="dimmed">{"\u2014"}</Text>
                    )}
                  </div>
                  <div>
                    <Text size="xs" c="dimmed">Next Run</Text>
                    {entry.next_run ? (
                      <Tooltip label={new Date(entry.next_run).toLocaleString()}>
                        <Text size="xs">{relativeTime(entry.next_run)}</Text>
                      </Tooltip>
                    ) : (
                      <Text size="xs" c="dimmed">{"\u2014"}</Text>
                    )}
                  </div>
                </SimpleGrid>

                {/* Last error */}
                {entry.last_err && (
                  <Text size="xs" c="red" truncate mt="xs">
                    {entry.last_err}
                  </Text>
                )}
              </Box>

              {/* Expandable execution history */}
              <Collapse in={expanded === entry.name}>
                <Box
                  style={{ borderTop: "1px solid var(--mantine-color-default-border)" }}
                  bg="var(--mantine-color-gray-light)"
                >
                  <Group gap="xs" px="md" py="xs">
                    <IconHistory size={14} style={{ opacity: 0.6 }} />
                    <Text size="xs" fw={500}>Execution History</Text>
                  </Group>
                  <RunHistory name={entry.name} />
                </Box>
              </Collapse>
            </Paper>
          ))}
        </Stack>
      )}
    </Stack>
  );
}
