import { useCallback, useEffect, useRef, useState } from "react";
import {
  ActionIcon,
  Alert,
  Badge,
  Box,
  Button,
  Group,
  Loader,
  NativeSelect,
  Paper,
  SegmentedControl,
  Stack,
  Table,
  Text,
  TextInput,
  Title,
} from "@mantine/core";
import {
  IconAlertCircle,
  IconAlertTriangle,
  IconBug,
  IconChevronDown,
  IconChevronRight,
  IconInfoCircle,
  IconPlayerPause,
  IconPlayerPlay,
  IconRefresh,
  IconSearch,
  IconWifi,
  IconWifiOff,
  IconX,
} from "@tabler/icons-react";
import { get } from "@/lib/api";

interface LogEntry {
  time: string;
  level: string;
  msg: string;
  [key: string]: unknown;
}

interface LogFile {
  name: string;
  size: number;
}

const LEVELS = [
  { value: "", label: "All" },
  { value: "debug", label: "Debug" },
  { value: "info", label: "Info" },
  { value: "warn", label: "Warn" },
  { value: "error", label: "Error" },
];

const MAX_ENTRIES = 500;

function formatTime(iso: string): string {
  if (!iso) return "\u2014";
  const d = new Date(iso);
  return d.toLocaleTimeString([], { hour12: false }) + "." + String(d.getMilliseconds()).padStart(3, "0");
}

function formatDate(iso: string): string {
  if (!iso) return "";
  return new Date(iso).toLocaleDateString();
}

function formatSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}

function wsUrl(path: string): string {
  const loc = window.location;
  const proto = loc.protocol === "https:" ? "wss:" : "ws:";
  return `${proto}//${loc.host}/api${path}`;
}

const levelColors: Record<string, string> = {
  debug: "gray",
  info: "blue",
  warn: "yellow",
  error: "red",
};

const levelIcons: Record<string, React.ReactNode> = {
  debug: <IconBug size={10} />,
  info: <IconInfoCircle size={10} />,
  warn: <IconAlertTriangle size={10} />,
  error: <IconAlertCircle size={10} />,
};

function ExtraFields({ entry }: { entry: LogEntry }) {
  const reserved = new Set(["time", "level", "msg"]);
  const extra = Object.entries(entry).filter(([k]) => !reserved.has(k));
  if (extra.length === 0) return <Text size="xs" c="dimmed">No extra fields</Text>;
  return (
    <Stack gap={4}>
      {extra.map(([k, v]) => (
        <Group key={k} gap="sm" wrap="nowrap">
          <Text size="xs" fw={500} c="dimmed">{k}:</Text>
          <Text size="xs" style={{ wordBreak: "break-all" }}>
            {typeof v === "string" ? v : JSON.stringify(v)}
          </Text>
        </Group>
      ))}
    </Stack>
  );
}

type WsState = "disconnected" | "connecting" | "connected";

export default function LogsPage() {
  const [entries, setEntries] = useState<LogEntry[]>([]);
  const [files, setFiles] = useState<LogFile[]>([]);
  const [selectedFile, setSelectedFile] = useState("stanza.log");
  const [level, setLevel] = useState("");
  const [search, setSearch] = useState("");
  const [searchInput, setSearchInput] = useState("");
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(true);
  const [expanded, setExpanded] = useState<Set<number>>(new Set());
  const [autoRefresh, setAutoRefresh] = useState(true);
  const [total, setTotal] = useState(0);
  const [wsState, setWsState] = useState<WsState>("disconnected");
  const [streamCount, setStreamCount] = useState(0);
  const intervalRef = useRef<ReturnType<typeof setInterval> | null>(null);
  const wsRef = useRef<WebSocket | null>(null);
  const reconnectRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  const loadFiles = useCallback(async () => {
    try {
      const data = await get<{ files: LogFile[] }>("/admin/logs/files");
      setFiles(data.files);
    } catch {
      // Files list is optional.
    }
  }, []);

  const loadEntries = useCallback(async () => {
    try {
      const params = new URLSearchParams({ limit: "200", file: selectedFile });
      if (level) params.set("level", level);
      if (search) params.set("search", search);
      const data = await get<{ entries: LogEntry[]; file: string; total: number }>(`/admin/logs?${params}`);
      setEntries(data.entries);
      setTotal(data.total);
      setError("");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load logs");
    }
  }, [selectedFile, level, search]);

  const loadAll = useCallback(async () => {
    setLoading(true);
    await Promise.all([loadEntries(), loadFiles()]);
    setLoading(false);
  }, [loadEntries, loadFiles]);

  const closeWs = useCallback(() => {
    if (reconnectRef.current) {
      clearTimeout(reconnectRef.current);
      reconnectRef.current = null;
    }
    if (wsRef.current) {
      wsRef.current.close();
      wsRef.current = null;
    }
    setWsState("disconnected");
  }, []);

  const connectWs = useCallback(() => {
    closeWs();
    setWsState("connecting");
    setStreamCount(0);

    const params = new URLSearchParams();
    if (level) params.set("level", level);
    if (search) params.set("search", search);
    const qs = params.toString();
    const url = wsUrl(`/admin/logs/stream${qs ? "?" + qs : ""}`);

    const ws = new WebSocket(url);
    wsRef.current = ws;

    ws.onopen = () => {
      setWsState("connected");
      setError("");
    };

    ws.onmessage = (event) => {
      try {
        const entry = JSON.parse(event.data) as LogEntry;
        setEntries((prev) => {
          const next = [entry, ...prev];
          return next.length > MAX_ENTRIES ? next.slice(0, MAX_ENTRIES) : next;
        });
        setTotal((prev) => prev + 1);
        setStreamCount((prev) => prev + 1);
      } catch {
        // Ignore non-JSON messages.
      }
    };

    ws.onclose = () => {
      setWsState("disconnected");
      wsRef.current = null;
      reconnectRef.current = setTimeout(() => {
        reconnectRef.current = null;
      }, 3000);
    };

    ws.onerror = () => {
      // onclose will fire after this.
    };
  }, [level, search, closeWs]);

  const sendFilters = useCallback(() => {
    if (wsRef.current && wsRef.current.readyState === WebSocket.OPEN) {
      wsRef.current.send(JSON.stringify({ level, search }));
    }
  }, [level, search]);

  // Initial load.
  useEffect(() => {
    loadAll();
  }, [loadAll]);

  // Manage live mode.
  useEffect(() => {
    if (intervalRef.current) {
      clearInterval(intervalRef.current);
      intervalRef.current = null;
    }

    if (!autoRefresh) {
      closeWs();
      return;
    }

    if (selectedFile === "stanza.log") {
      connectWs();
    } else {
      closeWs();
      intervalRef.current = setInterval(loadEntries, 5_000);
    }

    return () => {
      if (intervalRef.current) clearInterval(intervalRef.current);
      closeWs();
    };
  }, [autoRefresh, selectedFile, connectWs, closeWs, loadEntries]);

  // Send filter updates to active WebSocket.
  useEffect(() => {
    if (wsState === "connected") {
      sendFilters();
    }
  }, [level, search, wsState, sendFilters]);

  function toggleExpanded(idx: number) {
    setExpanded((prev) => {
      const next = new Set(prev);
      if (next.has(idx)) next.delete(idx);
      else next.add(idx);
      return next;
    });
  }

  function handleSearch(e: React.FormEvent) {
    e.preventDefault();
    setSearch(searchInput);
  }

  // File selector options.
  const fileOptions = files.length > 0
    ? files.map((f) => ({ value: f.name, label: `${f.name} (${formatSize(f.size)})` }))
    : [{ value: "stanza.log", label: "stanza.log" }];

  // Group entries by date.
  let lastDate = "";

  return (
    <Stack>
      {/* Header */}
      <Group justify="space-between">
        <div>
          <Title order={3}>Logs</Title>
          <Group gap="xs" mt={2}>
            <Text size="sm" c="dimmed">{total} entries in {selectedFile}</Text>
            {wsState === "connected" && streamCount > 0 && (
              <Text size="sm" c="green">+{streamCount} streamed</Text>
            )}
          </Group>
        </div>
        <Group gap="xs">
          {/* WebSocket status */}
          {wsState === "connected" && (
            <Badge variant="light" color="green" size="sm" leftSection={<IconWifi size={10} />}>
              Streaming
            </Badge>
          )}
          {wsState === "connecting" && (
            <Badge variant="light" color="yellow" size="sm" leftSection={<IconWifi size={10} />}>
              Connecting
            </Badge>
          )}
          {autoRefresh && selectedFile === "stanza.log" && wsState === "disconnected" && (
            <Badge variant="light" color="gray" size="sm" leftSection={<IconWifiOff size={10} />}>
              Disconnected
            </Badge>
          )}
          <Button
            variant={autoRefresh ? "filled" : "default"}
            size="xs"
            leftSection={autoRefresh ? <IconPlayerPause size={16} /> : <IconPlayerPlay size={16} />}
            onClick={() => setAutoRefresh(!autoRefresh)}
          >
            {autoRefresh ? "Live" : "Paused"}
          </Button>
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
      </Group>

      {/* Error */}
      {error && (
        <Alert icon={<IconAlertCircle size={16} />} color="red" variant="light" withCloseButton onClose={() => setError("")}>
          {error}
          <Button variant="subtle" size="xs" ml="sm" onClick={loadAll}>Retry</Button>
        </Alert>
      )}

      {/* Filters */}
      <Paper withBorder p="sm" radius="md">
        <Group gap="md" wrap="wrap">
          {/* Level filter */}
          <SegmentedControl
            value={level}
            onChange={setLevel}
            data={LEVELS}
            size="xs"
          />

          {/* Search */}
          <form onSubmit={handleSearch} style={{ display: "flex", gap: 8, alignItems: "center" }}>
            <TextInput
              placeholder="Search messages..."
              size="xs"
              leftSection={<IconSearch size={14} />}
              value={searchInput}
              onChange={(e) => setSearchInput(e.currentTarget.value)}
              w={180}
            />
            <Button type="submit" variant="default" size="xs">Search</Button>
            {search && (
              <ActionIcon variant="subtle" size="sm" onClick={() => { setSearch(""); setSearchInput(""); }}>
                <IconX size={14} />
              </ActionIcon>
            )}
          </form>

          {/* File selector */}
          <NativeSelect
            value={selectedFile}
            onChange={(e) => setSelectedFile(e.currentTarget.value)}
            data={fileOptions}
            size="xs"
            w={200}
          />
        </Group>
      </Paper>

      {/* Log entries */}
      <Paper withBorder radius="md" p={0}>
        <Group justify="space-between" px="md" py="sm" style={{ borderBottom: "1px solid var(--mantine-color-default-border)" }}>
          <Text size="sm" fw={500}>
            Log Entries
            {entries.length > 0 && (
              <Text span size="sm" c="dimmed" ml="xs">
                showing {entries.length} of {total}
              </Text>
            )}
          </Text>
        </Group>

        {loading && entries.length === 0 ? (
          <Group justify="center" py="xl">
            <Loader />
          </Group>
        ) : entries.length === 0 ? (
          <Text ta="center" c="dimmed" py="xl">No log entries found</Text>
        ) : (
          <Table.ScrollContainer minWidth={500}>
            <Table fz="sm">
              <Table.Thead>
                <Table.Tr>
                  <Table.Th w={32}></Table.Th>
                  <Table.Th>Time</Table.Th>
                  <Table.Th>Level</Table.Th>
                  <Table.Th>Message</Table.Th>
                </Table.Tr>
              </Table.Thead>
              <Table.Tbody>
                {entries.map((entry, idx) => {
                  const date = formatDate(entry.time);
                  const showDateSep = date !== lastDate;
                  lastDate = date;
                  const isExpanded = expanded.has(idx);

                  return (
                    <Box key={idx} component="tbody">
                      {/* Date separator */}
                      {showDateSep && (
                        <Table.Tr>
                          <Table.Td
                            colSpan={4}
                            bg="var(--mantine-color-gray-light)"
                            py={6}
                            px="md"
                          >
                            <Text size="xs" fw={500} c="dimmed">{date}</Text>
                          </Table.Td>
                        </Table.Tr>
                      )}

                      {/* Log entry row */}
                      <Table.Tr
                        style={{ cursor: "pointer" }}
                        onClick={() => toggleExpanded(idx)}
                      >
                        <Table.Td>
                          {isExpanded ? (
                            <IconChevronDown size={14} style={{ opacity: 0.5 }} />
                          ) : (
                            <IconChevronRight size={14} style={{ opacity: 0.5 }} />
                          )}
                        </Table.Td>
                        <Table.Td>
                          <Text size="xs" ff="monospace" c="dimmed" style={{ whiteSpace: "nowrap" }}>
                            {formatTime(entry.time)}
                          </Text>
                        </Table.Td>
                        <Table.Td>
                          <Badge
                            variant="light"
                            color={levelColors[entry.level] || "gray"}
                            size="xs"
                            w={60}
                            leftSection={levelIcons[entry.level]}
                            style={{ textAlign: "center" }}
                          >
                            {entry.level}
                          </Badge>
                        </Table.Td>
                        <Table.Td>
                          <Text size="sm" truncate maw={600}>{entry.msg}</Text>
                        </Table.Td>
                      </Table.Tr>

                      {/* Expanded detail row */}
                      {isExpanded && (
                        <Table.Tr bg="var(--mantine-color-gray-light)">
                          <Table.Td></Table.Td>
                          <Table.Td colSpan={3} py="sm">
                            <ExtraFields entry={entry} />
                          </Table.Td>
                        </Table.Tr>
                      )}
                    </Box>
                  );
                })}
              </Table.Tbody>
            </Table>
          </Table.ScrollContainer>
        )}
      </Paper>
    </Stack>
  );
}
