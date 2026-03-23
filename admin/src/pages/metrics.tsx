import { useCallback, useEffect, useRef, useState } from "react";
import {
  ActionIcon,
  Alert,
  Badge,
  Button,
  Card,
  Group,
  Loader,
  Menu,
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
  IconCheck,
  IconClockHour4,
  IconDatabase,
  IconDeviceFloppy,
  IconDotsVertical,
  IconLayoutDashboard,
  IconPlus,
  IconRefresh,
  IconTrash,
  IconX,
} from "@tabler/icons-react";
import { del, get, post, put } from "@/lib/api";

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

interface PanelConfig {
  metric: string;
  range: string;
  fn: string;
  labels: string;
}

interface Dashboard {
  id: number;
  name: string;
  panels: PanelConfig[];
  created_at: string;
  updated_at: string;
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
    color: CHART_COLORS[i % CHART_COLORS.length]!,
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
      const v = lookups[i]!.get(ts);
      row[labels[i]!] = v !== undefined ? Math.round(v * 1000) / 1000 : null;
    }
    return row;
  });

  return { data, seriesConfig };
}

let nextPanelId = 1;

function MetricChartPanel({
  names,
  initialConfig,
  onConfigChange,
  onRemove,
  canRemove,
}: {
  names: string[];
  initialConfig?: PanelConfig;
  onConfigChange?: (config: PanelConfig) => void;
  onRemove: () => void;
  canRemove: boolean;
}) {
  const [metric, setMetric] = useState(initialConfig?.metric ?? "");
  const [range, setRange] = useState(initialConfig?.range ?? "1h");
  const [fn, setFn] = useState(initialConfig?.fn ?? "sum");
  const [labels, setLabels] = useState(initialConfig?.labels ?? "");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const [series, setSeries] = useState<SeriesData[]>([]);

  const labelsRef = useRef(labels);
  labelsRef.current = labels;

  // Report config changes to parent for dashboard save.
  const configRef = useRef(onConfigChange);
  configRef.current = onConfigChange;
  useEffect(() => {
    configRef.current?.({ metric, range, fn, labels });
  }, [metric, range, fn, labels]);

  const runQuery = useCallback(async () => {
    if (!metric) return;
    setLoading(true);
    setError("");
    try {
      const tr = TIME_RANGES.find((r) => r.value === range) ?? TIME_RANGES[0]!;
      const end = new Date();
      const start = new Date(end.getTime() - tr!.ms);
      const params = new URLSearchParams({
        name: metric,
        start: start.toISOString(),
        end: end.toISOString(),
        step: tr!.step,
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
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  // Panel state: each panel has an id and an optional initial config.
  const [panels, setPanels] = useState<{ id: number; config?: PanelConfig }[]>([
    { id: nextPanelId++ },
  ]);

  // Dashboard state.
  const [dashboards, setDashboards] = useState<Dashboard[]>([]);
  const [activeDashboardId, setActiveDashboardId] = useState<number | null>(null);
  const [saveName, setSaveName] = useState("");
  const [saving, setSaving] = useState(false);
  const [saveSuccess, setSaveSuccess] = useState(false);

  // Track current panel configs via refs (updated by child callbacks).
  const panelConfigsRef = useRef<Map<number, PanelConfig>>(new Map());

  useEffect(() => {
    (async () => {
      try {
        const [statsRes, namesRes, dashRes] = await Promise.all([
          get<StoreStats>("/admin/metrics/stats"),
          get<{ names: string[] }>("/admin/metrics/names"),
          get<{ dashboards: Dashboard[] }>("/admin/dashboards"),
        ]);
        setStats(statsRes);
        setNames(namesRes.names ?? []);
        setDashboards(dashRes.dashboards ?? []);
      } catch {
        setError("Failed to load metrics");
      } finally {
        setLoading(false);
      }
    })();
  }, []);

  const addPanel = useCallback(() => {
    setPanels((prev) => [...prev, { id: nextPanelId++ }]);
  }, []);

  const removePanel = useCallback((id: number) => {
    setPanels((prev) => {
      if (prev.length <= 1) return prev;
      panelConfigsRef.current.delete(id);
      return prev.filter((p) => p.id !== id);
    });
  }, []);

  const handleConfigChange = useCallback((panelId: number, config: PanelConfig) => {
    panelConfigsRef.current.set(panelId, config);
  }, []);

  // Collect current panel configs from the ref map.
  const collectPanelConfigs = useCallback((): PanelConfig[] => {
    return panels.map((p) => panelConfigsRef.current.get(p.id) ?? {
      metric: "", range: "1h", fn: "sum", labels: "",
    });
  }, [panels]);

  // Load a saved dashboard.
  const loadDashboard = useCallback((dashboard: Dashboard) => {
    panelConfigsRef.current.clear();
    const newPanels: { id: number; config?: PanelConfig }[] = dashboard.panels.map((config) => {
      const id = nextPanelId++;
      panelConfigsRef.current.set(id, config);
      return { id, config };
    });
    if (newPanels.length === 0) {
      newPanels.push({ id: nextPanelId++ });
    }
    setPanels(newPanels);
    setActiveDashboardId(dashboard.id);
    setSaveName(dashboard.name);
  }, []);

  // Switch to explorer mode (no dashboard loaded).
  const switchToExplorer = useCallback(() => {
    panelConfigsRef.current.clear();
    setPanels([{ id: nextPanelId++ }]);
    setActiveDashboardId(null);
    setSaveName("");
  }, []);

  // Save current panels as a new dashboard or update existing.
  const saveDashboard = useCallback(async () => {
    const name = saveName.trim();
    if (!name) return;

    setSaving(true);
    try {
      const panelConfigs = collectPanelConfigs();
      if (activeDashboardId) {
        // Update existing dashboard.
        const updated = await put<Dashboard>(`/admin/dashboards/${activeDashboardId}`, {
          name,
          panels: panelConfigs,
        });
        setDashboards((prev) => prev.map((d) => (d.id === updated.id ? updated : d)));
      } else {
        // Create new dashboard.
        const created = await post<Dashboard>("/admin/dashboards", {
          name,
          panels: panelConfigs,
        });
        setDashboards((prev) => [created, ...prev]);
        setActiveDashboardId(created.id);
      }
      setSaveSuccess(true);
      setTimeout(() => setSaveSuccess(false), 2000);
    } catch {
      // Silently fail — the user can retry.
    } finally {
      setSaving(false);
    }
  }, [saveName, activeDashboardId, collectPanelConfigs]);

  // Delete a dashboard.
  const deleteDashboard = useCallback(async (id: number) => {
    try {
      await del(`/admin/dashboards/${id}`);
      setDashboards((prev) => prev.filter((d) => d.id !== id));
      if (activeDashboardId === id) {
        switchToExplorer();
      }
    } catch {
      // Silently fail.
    }
  }, [activeDashboardId, switchToExplorer]);

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

  const activeDashboard = dashboards.find((d) => d.id === activeDashboardId);

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

      {/* Dashboard bar */}
      <Paper withBorder p="sm" radius="md">
        <Group justify="space-between" wrap="wrap" gap="xs">
          <Group gap="xs" wrap="wrap">
            <IconLayoutDashboard size={18} style={{ opacity: 0.6 }} />
            {dashboards.length > 0 ? (
              <Menu shadow="md" width={220}>
                <Menu.Target>
                  <Button variant="subtle" size="compact-xs">
                    {activeDashboard ? activeDashboard.name : "Explorer"}
                  </Button>
                </Menu.Target>
                <Menu.Dropdown>
                  <Menu.Item onClick={switchToExplorer}>
                    Explorer
                  </Menu.Item>
                  {dashboards.length > 0 && <Menu.Divider />}
                  {dashboards.map((d) => (
                    <Menu.Item
                      key={d.id}
                      onClick={() => loadDashboard(d)}
                      rightSection={
                        <ActionIcon
                          size="xs"
                          variant="subtle"
                          color="red"
                          onClick={(e) => {
                            e.stopPropagation();
                            deleteDashboard(d.id);
                          }}
                        >
                          <IconTrash size={12} />
                        </ActionIcon>
                      }
                    >
                      {d.name}
                    </Menu.Item>
                  ))}
                </Menu.Dropdown>
              </Menu>
            ) : (
              <Text size="sm" c="dimmed">Explorer</Text>
            )}
          </Group>
          <Group gap="xs">
            <TextInput
              size="xs"
              w={180}
              placeholder="Dashboard name..."
              value={saveName}
              onChange={(e) => setSaveName(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === "Enter") saveDashboard();
              }}
            />
            <Tooltip label={activeDashboardId ? "Update dashboard" : "Save as new dashboard"}>
              <ActionIcon
                variant="light"
                size="sm"
                onClick={saveDashboard}
                loading={saving}
                disabled={!saveName.trim()}
                color={saveSuccess ? "green" : undefined}
              >
                {saveSuccess ? <IconCheck size={14} /> : <IconDeviceFloppy size={14} />}
              </ActionIcon>
            </Tooltip>
            {activeDashboardId && (
              <Tooltip label="Save as new dashboard">
                <ActionIcon
                  variant="light"
                  size="sm"
                  onClick={() => {
                    setActiveDashboardId(null);
                    saveDashboard();
                  }}
                >
                  <IconDotsVertical size={14} />
                </ActionIcon>
              </Tooltip>
            )}
          </Group>
        </Group>
      </Paper>

      {panels.map((panel) => (
        <MetricChartPanel
          key={panel.id}
          names={names}
          initialConfig={panel.config}
          onConfigChange={(config) => handleConfigChange(panel.id, config)}
          onRemove={() => removePanel(panel.id)}
          canRemove={panels.length > 1}
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
