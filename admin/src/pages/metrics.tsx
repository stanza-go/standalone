import { useCallback, useEffect, useRef, useState } from "react";
import {
  ActionIcon,
  Alert,
  Badge,
  Card,
  Group,
  Loader,
  NativeSelect,
  Paper,
  SegmentedControl,
  SimpleGrid,
  Stack,
  Text,
  TextInput,
  ThemeIcon,
  Title,
  Tooltip,
} from "@mantine/core";
import { AreaChart } from "@mantine/charts";
import {
  IconAlertCircle,
  IconChartLine,
  IconClockHour4,
  IconDatabase,
  IconPlus,
  IconRefresh,
  IconTrash,
  IconX,
} from "@tabler/icons-react";
import { get } from "@/lib/api";

interface StoreStats {
  series_count: number;
  partition_count: number;
  disk_bytes: number;
  oldest_date: string;
  newest_date: string;
}

interface SeriesPoint {
  t: number;
  v: number;
}

interface SeriesData {
  name: string;
  labels: Record<string, string>;
  points: SeriesPoint[];
}

interface QueryResponse {
  series: SeriesData[];
}

const TIME_RANGES = [
  { label: "1h", value: "1h", ms: 3_600_000, step: "1m" },
  { label: "6h", value: "6h", ms: 21_600_000, step: "5m" },
  { label: "24h", value: "24h", ms: 86_400_000, step: "15m" },
  { label: "7d", value: "7d", ms: 604_800_000, step: "1h" },
  { label: "30d", value: "30d", ms: 2_592_000_000, step: "6h" },
];

const AGG_FUNCTIONS = [
  { label: "Sum", value: "sum" },
  { label: "Avg", value: "avg" },
  { label: "Min", value: "min" },
  { label: "Max", value: "max" },
  { label: "Count", value: "count" },
  { label: "Last", value: "last" },
];

const CHART_COLORS = [
  "blue.6", "teal.6", "orange.6", "violet.6", "red.6",
  "cyan.6", "green.6", "pink.6", "yellow.6", "indigo.6",
];

function formatBytes(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}

function seriesLabel(s: SeriesData): string {
  const entries = Object.entries(s.labels || {});
  if (entries.length === 0) return s.name;
  return entries.map(([k, v]) => `${k}=${v}`).join(", ");
}

function formatTime(ts: number, range: string): string {
  const d = new Date(ts);
  if (range === "1h" || range === "6h" || range === "24h") {
    return d.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" });
  }
  if (range === "7d") {
    const mm = String(d.getMonth() + 1).padStart(2, "0");
    const dd = String(d.getDate()).padStart(2, "0");
    const hh = String(d.getHours()).padStart(2, "0");
    return `${mm}/${dd} ${hh}:00`;
  }
  const mm = String(d.getMonth() + 1).padStart(2, "0");
  const dd = String(d.getDate()).padStart(2, "0");
  return `${mm}/${dd}`;
}

function toChartData(
  series: SeriesData[],
  range: string,
): { data: Record<string, unknown>[]; seriesConfig: { name: string; color: string }[] } {
  if (series.length === 0) return { data: [], seriesConfig: [] };

  const tsSet = new Set<number>();
  for (const s of series) {
    for (const p of s.points) tsSet.add(p.t);
  }
  const timestamps = [...tsSet].sort((a, b) => a - b);

  const labels = series.map(seriesLabel);
  const seriesConfig = labels.map((name, i) => ({
    name,
    color: CHART_COLORS[i % CHART_COLORS.length],
  }));

  // Build a lookup map per series for O(1) point access.
  const lookups = series.map((s) => {
    const m = new Map<number, number>();
    for (const p of s.points) m.set(p.t, p.v);
    return m;
  });

  const data = timestamps.map((ts) => {
    const row: Record<string, unknown> = { time: formatTime(ts, range) };
    for (let i = 0; i < series.length; i++) {
      const v = lookups[i].get(ts);
      row[labels[i]] = v !== undefined ? Math.round(v * 1000) / 1000 : null;
    }
    return row;
  });

  return { data, seriesConfig };
}

let nextPanelId = 1;

function MetricChartPanel({
  names,
  onRemove,
  canRemove,
}: {
  names: string[];
  onRemove: () => void;
  canRemove: boolean;
}) {
  const [metric, setMetric] = useState("");
  const [range, setRange] = useState("1h");
  const [fn, setFn] = useState("sum");
  const [labels, setLabels] = useState("");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const [series, setSeries] = useState<SeriesData[]>([]);

  const labelsRef = useRef(labels);
  labelsRef.current = labels;

  const runQuery = useCallback(async () => {
    if (!metric) return;
    setLoading(true);
    setError("");
    try {
      const tr = TIME_RANGES.find((r) => r.value === range) ?? TIME_RANGES[0];
      const end = new Date();
      const start = new Date(end.getTime() - tr.ms);
      const params = new URLSearchParams({
        name: metric,
        start: start.toISOString(),
        end: end.toISOString(),
        step: tr.step,
        fn,
      });
      if (labelsRef.current.trim()) {
        params.set("labels", labelsRef.current.trim());
      }
      const res = await get<QueryResponse>(`/admin/metrics/query?${params}`);
      setSeries(res.series ?? []);
    } catch {
      setError("Query failed");
    } finally {
      setLoading(false);
    }
  }, [metric, range, fn]);

  useEffect(() => {
    runQuery();
  }, [runQuery]);

  const { data, seriesConfig } = toChartData(series, range);

  return (
    <Paper withBorder p="md" radius="md">
      <Stack gap="sm">
        <Group justify="space-between" wrap="wrap" gap="xs">
          <Group wrap="wrap" gap="xs">
            <NativeSelect
              size="xs"
              w={200}
              value={metric}
              onChange={(e) => setMetric(e.target.value)}
              data={[{ label: "Select metric...", value: "" }, ...names]}
            />
            <SegmentedControl
              size="xs"
              value={range}
              onChange={setRange}
              data={TIME_RANGES.map((r) => ({ label: r.label, value: r.value }))}
            />
            <NativeSelect
              size="xs"
              w={100}
              value={fn}
              onChange={(e) => setFn(e.target.value)}
              data={AGG_FUNCTIONS}
            />
            <TextInput
              size="xs"
              w={240}
              placeholder="Labels: method=GET,status=200"
              value={labels}
              onChange={(e) => setLabels(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === "Enter") runQuery();
              }}
              rightSection={
                labels ? (
                  <ActionIcon
                    size="xs"
                    variant="subtle"
                    onClick={() => {
                      setLabels("");
                      labelsRef.current = "";
                      runQuery();
                    }}
                  >
                    <IconX size={12} />
                  </ActionIcon>
                ) : null
              }
            />
          </Group>
          <Group gap="xs">
            {series.length > 0 && (
              <Badge size="sm" variant="light">
                {series.length} series
              </Badge>
            )}
            <Tooltip label="Refresh">
              <ActionIcon
                variant="light"
                size="sm"
                onClick={runQuery}
                loading={loading}
                disabled={!metric}
              >
                <IconRefresh size={14} />
              </ActionIcon>
            </Tooltip>
            {canRemove && (
              <Tooltip label="Remove chart">
                <ActionIcon variant="light" color="red" size="sm" onClick={onRemove}>
                  <IconTrash size={14} />
                </ActionIcon>
              </Tooltip>
            )}
          </Group>
        </Group>

        {error && (
          <Alert icon={<IconAlertCircle size={14} />} color="red" variant="light" py="xs">
            {error}
          </Alert>
        )}

        {loading && (
          <Group justify="center" py="xl">
            <Loader size="sm" />
          </Group>
        )}

        {!loading && !error && metric && data.length > 0 && (
          <AreaChart
            h={260}
            data={data}
            dataKey="time"
            series={seriesConfig}
            curveType="monotone"
            withDots={false}
            withGradient
            gridAxis="xy"
            tickLine="xy"
            valueFormatter={(v) =>
              typeof v === "number" ? (v >= 1000 ? `${(v / 1000).toFixed(1)}k` : String(v)) : ""
            }
          />
        )}

        {!loading && !error && metric && data.length === 0 && (
          <Text c="dimmed" ta="center" py="xl" size="sm">
            No data for this query. Try a different time range or metric.
          </Text>
        )}

        {!metric && (
          <Text c="dimmed" ta="center" py="xl" size="sm">
            Select a metric to start exploring.
          </Text>
        )}
      </Stack>
    </Paper>
  );
}

export default function MetricsPage() {
  const [stats, setStats] = useState<StoreStats | null>(null);
  const [names, setNames] = useState<string[]>([]);
  const [panelIds, setPanelIds] = useState<number[]>([nextPanelId++]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  useEffect(() => {
    (async () => {
      try {
        const [statsRes, namesRes] = await Promise.all([
          get<StoreStats>("/admin/metrics/stats"),
          get<{ names: string[] }>("/admin/metrics/names"),
        ]);
        setStats(statsRes);
        setNames(namesRes.names ?? []);
      } catch {
        setError("Failed to load metrics");
      } finally {
        setLoading(false);
      }
    })();
  }, []);

  const addPanel = useCallback(() => {
    setPanelIds((prev) => [...prev, nextPanelId++]);
  }, []);

  const removePanel = useCallback((id: number) => {
    setPanelIds((prev) => (prev.length > 1 ? prev.filter((p) => p !== id) : prev));
  }, []);

  if (error) {
    return (
      <Alert icon={<IconAlertCircle size={16} />} color="red" variant="light">
        {error}
      </Alert>
    );
  }

  if (loading) {
    return (
      <Group justify="center" pt="xl">
        <Loader />
      </Group>
    );
  }

  return (
    <Stack>
      <Group justify="space-between">
        <Title order={3}>Metrics</Title>
        <Badge variant="light" size="lg">
          {names.length} metrics
        </Badge>
      </Group>

      {stats && (
        <SimpleGrid cols={{ base: 1, xs: 2, md: 4 }}>
          <Card withBorder padding="lg" radius="md">
            <Group justify="space-between" wrap="nowrap">
              <div>
                <Text size="xs" c="dimmed" tt="uppercase" fw={700}>Series</Text>
                <Text fw={700} size="xl" mt={4}>{stats.series_count}</Text>
              </div>
              <ThemeIcon size={48} radius="md" variant="light" color="blue">
                <IconChartLine size={24} stroke={1.5} />
              </ThemeIcon>
            </Group>
          </Card>
          <Card withBorder padding="lg" radius="md">
            <Group justify="space-between" wrap="nowrap">
              <div>
                <Text size="xs" c="dimmed" tt="uppercase" fw={700}>Partitions</Text>
                <Text fw={700} size="xl" mt={4}>{stats.partition_count}</Text>
              </div>
              <ThemeIcon size={48} radius="md" variant="light" color="teal">
                <IconDatabase size={24} stroke={1.5} />
              </ThemeIcon>
            </Group>
          </Card>
          <Card withBorder padding="lg" radius="md">
            <Group justify="space-between" wrap="nowrap">
              <div>
                <Text size="xs" c="dimmed" tt="uppercase" fw={700}>Disk Usage</Text>
                <Text fw={700} size="xl" mt={4}>{formatBytes(stats.disk_bytes)}</Text>
              </div>
              <ThemeIcon size={48} radius="md" variant="light" color="violet">
                <IconDatabase size={24} stroke={1.5} />
              </ThemeIcon>
            </Group>
          </Card>
          <Card withBorder padding="lg" radius="md">
            <Group justify="space-between" wrap="nowrap">
              <div>
                <Text size="xs" c="dimmed" tt="uppercase" fw={700}>Data Range</Text>
                <Text fw={700} size="lg" mt={4}>
                  {stats.oldest_date && stats.newest_date
                    ? `${stats.oldest_date} — ${stats.newest_date}`
                    : "No data"}
                </Text>
              </div>
              <ThemeIcon size={48} radius="md" variant="light" color="orange">
                <IconClockHour4 size={24} stroke={1.5} />
              </ThemeIcon>
            </Group>
          </Card>
        </SimpleGrid>
      )}

      {panelIds.map((id) => (
        <MetricChartPanel
          key={id}
          names={names}
          onRemove={() => removePanel(id)}
          canRemove={panelIds.length > 1}
        />
      ))}

      <Group>
        <Tooltip label="Add another chart">
          <ActionIcon variant="light" size="lg" onClick={addPanel}>
            <IconPlus size={18} />
          </ActionIcon>
        </Tooltip>
      </Group>
    </Stack>
  );
}
