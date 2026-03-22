import { useCallback, useEffect, useState } from "react";
import {
  Alert,
  Badge,
  Card,
  Grid,
  Group,
  Loader,
  SegmentedControl,
  SimpleGrid,
  Stack,
  Text,
  ThemeIcon,
  Timeline,
  Title,
} from "@mantine/core";
import { AreaChart, BarChart } from "@mantine/charts";
import {
  IconAlertCircle,
  IconClock,
  IconDatabase,
  IconServer,
  IconUsers,
} from "@tabler/icons-react";
import { get } from "@/lib/api";

interface DashboardData {
  system: {
    uptime: string;
    uptime_seconds: number;
    goroutines: number;
    memory_alloc_mb: number;
    memory_sys_mb: number;
    go_version: string;
  };
  database: {
    size_bytes: number;
    wal_size_bytes: number;
    tables: number;
    migrations: number;
  };
  queue: {
    pending: number;
    running: number;
    completed: number;
    failed: number;
    dead: number;
    cancelled: number;
  };
  cron: {
    total: number;
    enabled: number;
    running: number;
    next_run: string;
  };
  stats: {
    total_admins: number;
    total_users: number;
    active_sessions: number;
    active_api_keys: number;
  };
}

interface ChartsData {
  users: { date: string; count: number }[];
  activity: { date: string; count: number }[];
  jobs: { date: string; completed: number; failed: number }[];
}

interface ActivityEntry {
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

function StatCard({
  title,
  value,
  icon: Icon,
  color,
}: {
  title: string;
  value: string | number;
  icon: React.FC<{ size?: number; stroke?: number }>;
  color: string;
}) {
  return (
    <Card withBorder padding="lg" radius="md">
      <Group justify="space-between" wrap="nowrap">
        <div>
          <Text size="xs" c="dimmed" tt="uppercase" fw={700}>
            {title}
          </Text>
          <Text fw={700} size="xl" mt={4}>
            {value}
          </Text>
        </div>
        <ThemeIcon size={48} radius="md" variant="light" color={color}>
          <Icon size={24} stroke={1.5} />
        </ThemeIcon>
      </Group>
    </Card>
  );
}

function formatBytes(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}

function formatDate(dateStr: string): string {
  const parts = dateStr.split("-");
  if (parts.length === 3) {
    return `${parts[1]}/${parts[2]}`;
  }
  return dateStr;
}

function timeAgo(dateStr: string): string {
  const now = Date.now();
  const then = new Date(dateStr).getTime();
  const seconds = Math.floor((now - then) / 1000);
  if (seconds < 60) return "just now";
  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) return `${minutes}m ago`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours}h ago`;
  const days = Math.floor(hours / 24);
  return `${days}d ago`;
}

const ACTION_COLORS: Record<string, string> = {
  create: "green",
  update: "blue",
  delete: "red",
  login: "cyan",
  logout: "gray",
  revoke: "orange",
  export: "violet",
  import: "violet",
};

function actionColor(action: string): string {
  for (const [key, color] of Object.entries(ACTION_COLORS)) {
    if (action.toLowerCase().includes(key)) return color;
  }
  return "gray";
}

export default function DashboardPage() {
  const [data, setData] = useState<DashboardData | null>(null);
  const [charts, setCharts] = useState<ChartsData | null>(null);
  const [activity, setActivity] = useState<ActivityEntry[]>([]);
  const [period, setPeriod] = useState("7d");
  const [error, setError] = useState("");

  const loadStats = useCallback(async () => {
    try {
      setData(await get<DashboardData>("/admin/dashboard"));
    } catch {
      setError("Failed to load dashboard");
    }
  }, []);

  const loadCharts = useCallback(async (p: string) => {
    try {
      setCharts(await get<ChartsData>(`/admin/dashboard/charts?period=${p}`));
    } catch {
      // Charts are non-critical — silently fail.
    }
  }, []);

  const loadActivity = useCallback(async () => {
    try {
      const res = await get<{ entries: ActivityEntry[] }>("/admin/audit/recent");
      setActivity(res.entries);
    } catch {
      // Activity feed is non-critical — silently fail.
    }
  }, []);

  useEffect(() => {
    loadStats();
    loadActivity();
  }, [loadStats, loadActivity]);

  useEffect(() => {
    loadCharts(period);
  }, [period, loadCharts]);

  if (error) {
    return (
      <Alert icon={<IconAlertCircle size={16} />} color="red" variant="light">
        {error}
      </Alert>
    );
  }

  if (!data) {
    return (
      <Group justify="center" pt="xl">
        <Loader />
      </Group>
    );
  }

  const userChartData = charts?.users.map((d) => ({ date: formatDate(d.date), Users: d.count })) ?? [];
  const activityChartData = charts?.activity.map((d) => ({ date: formatDate(d.date), Actions: d.count })) ?? [];
  const jobChartData = charts?.jobs.map((d) => ({ date: formatDate(d.date), Completed: d.completed, Failed: d.failed })) ?? [];

  return (
    <Stack>
      <Title order={3}>Dashboard</Title>

      <SimpleGrid cols={{ base: 1, xs: 2, md: 4 }}>
        <StatCard title="Users" value={data.stats.total_users} icon={IconUsers} color="blue" />
        <StatCard title="Active Sessions" value={data.stats.active_sessions} icon={IconServer} color="green" />
        <StatCard title="Database" value={formatBytes(data.database.size_bytes)} icon={IconDatabase} color="violet" />
        <StatCard title="Uptime" value={data.system.uptime} icon={IconClock} color="orange" />
      </SimpleGrid>

      {charts && (
        <>
          <Group justify="space-between" align="center">
            <Text fw={600}>Trends</Text>
            <SegmentedControl
              size="xs"
              value={period}
              onChange={setPeriod}
              data={[
                { label: "7 days", value: "7d" },
                { label: "30 days", value: "30d" },
                { label: "90 days", value: "90d" },
              ]}
            />
          </Group>

          <Grid>
            <Grid.Col span={{ base: 12, md: 4 }}>
              <Card withBorder padding="lg" radius="md">
                <Text size="sm" c="dimmed" mb="sm">New Users</Text>
                <AreaChart
                  h={180}
                  data={userChartData}
                  dataKey="date"
                  series={[{ name: "Users", color: "blue.6" }]}
                  curveType="monotone"
                  withDots={false}
                  withGradient
                  gridAxis="x"
                  tickLine="none"
                  withXAxis={false}
                  withYAxis={false}
                />
              </Card>
            </Grid.Col>

            <Grid.Col span={{ base: 12, md: 4 }}>
              <Card withBorder padding="lg" radius="md">
                <Text size="sm" c="dimmed" mb="sm">Admin Activity</Text>
                <AreaChart
                  h={180}
                  data={activityChartData}
                  dataKey="date"
                  series={[{ name: "Actions", color: "teal.6" }]}
                  curveType="monotone"
                  withDots={false}
                  withGradient
                  gridAxis="x"
                  tickLine="none"
                  withXAxis={false}
                  withYAxis={false}
                />
              </Card>
            </Grid.Col>

            <Grid.Col span={{ base: 12, md: 4 }}>
              <Card withBorder padding="lg" radius="md">
                <Text size="sm" c="dimmed" mb="sm">Job Queue</Text>
                <BarChart
                  h={180}
                  data={jobChartData}
                  dataKey="date"
                  series={[
                    { name: "Completed", color: "green.6" },
                    { name: "Failed", color: "red.6" },
                  ]}
                  tickLine="none"
                  withXAxis={false}
                  withYAxis={false}
                />
              </Card>
            </Grid.Col>
          </Grid>
        </>
      )}

      <Grid>
        <Grid.Col span={{ base: 12, md: 6 }}>
          <Card withBorder padding="lg" radius="md">
            <Text fw={600} mb="sm">System</Text>
            <Stack gap={4}>
              <Group justify="space-between">
                <Text size="sm" c="dimmed">Go Version</Text>
                <Text size="sm">{data.system.go_version}</Text>
              </Group>
              <Group justify="space-between">
                <Text size="sm" c="dimmed">Goroutines</Text>
                <Text size="sm">{data.system.goroutines}</Text>
              </Group>
              <Group justify="space-between">
                <Text size="sm" c="dimmed">Memory (alloc)</Text>
                <Text size="sm">{data.system.memory_alloc_mb.toFixed(1)} MB</Text>
              </Group>
              <Group justify="space-between">
                <Text size="sm" c="dimmed">Memory (sys)</Text>
                <Text size="sm">{data.system.memory_sys_mb.toFixed(1)} MB</Text>
              </Group>
            </Stack>
          </Card>
        </Grid.Col>

        <Grid.Col span={{ base: 12, md: 6 }}>
          <Card withBorder padding="lg" radius="md">
            <Text fw={600} mb="sm">Database</Text>
            <Stack gap={4}>
              <Group justify="space-between">
                <Text size="sm" c="dimmed">Tables</Text>
                <Text size="sm">{data.database.tables}</Text>
              </Group>
              <Group justify="space-between">
                <Text size="sm" c="dimmed">Migrations</Text>
                <Text size="sm">{data.database.migrations}</Text>
              </Group>
              <Group justify="space-between">
                <Text size="sm" c="dimmed">DB Size</Text>
                <Text size="sm">{formatBytes(data.database.size_bytes)}</Text>
              </Group>
              <Group justify="space-between">
                <Text size="sm" c="dimmed">WAL Size</Text>
                <Text size="sm">{formatBytes(data.database.wal_size_bytes)}</Text>
              </Group>
            </Stack>
          </Card>
        </Grid.Col>

        <Grid.Col span={{ base: 12, md: 6 }}>
          <Card withBorder padding="lg" radius="md">
            <Text fw={600} mb="sm">Job Queue</Text>
            <Stack gap={4}>
              <Group justify="space-between">
                <Text size="sm" c="dimmed">Pending</Text>
                <Text size="sm">{data.queue.pending}</Text>
              </Group>
              <Group justify="space-between">
                <Text size="sm" c="dimmed">Running</Text>
                <Text size="sm">{data.queue.running}</Text>
              </Group>
              <Group justify="space-between">
                <Text size="sm" c="dimmed">Completed</Text>
                <Text size="sm">{data.queue.completed}</Text>
              </Group>
              <Group justify="space-between">
                <Text size="sm" c="dimmed">Failed</Text>
                <Text size="sm" c={data.queue.failed > 0 ? "red" : undefined}>{data.queue.failed}</Text>
              </Group>
            </Stack>
          </Card>
        </Grid.Col>

        <Grid.Col span={{ base: 12, md: 6 }}>
          <Card withBorder padding="lg" radius="md">
            <Text fw={600} mb="sm">Application</Text>
            <Stack gap={4}>
              <Group justify="space-between">
                <Text size="sm" c="dimmed">Admins</Text>
                <Text size="sm">{data.stats.total_admins}</Text>
              </Group>
              <Group justify="space-between">
                <Text size="sm" c="dimmed">API Keys</Text>
                <Text size="sm">{data.stats.active_api_keys}</Text>
              </Group>
              <Group justify="space-between">
                <Text size="sm" c="dimmed">Cron Jobs</Text>
                <Text size="sm">{data.cron.enabled} / {data.cron.total}</Text>
              </Group>
            </Stack>
          </Card>
        </Grid.Col>
      </Grid>

      {activity.length > 0 && (
        <Card withBorder padding="lg" radius="md">
          <Text fw={600} mb="md">Recent Activity</Text>
          <Timeline bulletSize={24} lineWidth={2}>
            {activity.map((entry) => (
              <Timeline.Item key={entry.id} bullet={<Text size="xs" fw={700}>{(entry.admin_name || entry.admin_email || "?").charAt(0).toUpperCase()}</Text>}>
                <Group gap="xs" wrap="wrap">
                  <Text size="sm" fw={500}>{entry.admin_name || entry.admin_email}</Text>
                  <Badge size="sm" variant="light" color={actionColor(entry.action)}>{entry.action}</Badge>
                  {entry.entity_type && (
                    <Text size="sm" c="dimmed">{entry.entity_type}{entry.entity_id ? ` #${entry.entity_id}` : ""}</Text>
                  )}
                </Group>
                {entry.details && <Text size="xs" c="dimmed" mt={2}>{entry.details}</Text>}
                <Text size="xs" c="dimmed" mt={2}>{timeAgo(entry.created_at)}</Text>
              </Timeline.Item>
            ))}
          </Timeline>
        </Card>
      )}
    </Stack>
  );
}
