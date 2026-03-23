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
  IconMail,
  IconServer,
  IconShieldCheck,
  IconUsers,
  IconWebhook,
  IconWorld,
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
  http: {
    total_requests: number;
    active_requests: number;
    status_2xx: number;
    status_3xx: number;
    status_4xx: number;
    status_5xx: number;
    bytes_written: number;
    avg_duration_ms: number;
  };
  auth: {
    issued: number;
    accepted: number;
    rejected: number;
  };
  webhook: {
    sends: number;
    successes: number;
    failures: number;
    retries: number;
    errors: number;
  };
  email: {
    sent: number;
    errors: number;
  };
  cache: {
    entries: number;
    hits: number;
    misses: number;
    evictions: number;
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

interface SeriesPoint {
  t: number;
  v: number;
}

interface QueryResponse {
  series: {
    name: string;
    labels: Record<string, string>;
    points: SeriesPoint[];
  }[];
}

interface TrafficPoint {
  time: string;
  Requests: number;
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

function InfoRow({ label, value, valueColor }: { label: string; value: string | number; valueColor?: string }) {
  return (
    <Group justify="space-between">
      <Text size="sm" c="dimmed">{label}</Text>
      <Text size="sm" c={valueColor}>{value}</Text>
    </Group>
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

function formatNumber(n: number): string {
  if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(1)}M`;
  if (n >= 1_000) return `${(n / 1_000).toFixed(1)}K`;
  return String(n);
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
  const [traffic, setTraffic] = useState<TrafficPoint[]>([]);
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

  const loadTraffic = useCallback(async () => {
    try {
      const end = new Date();
      const start = new Date(end.getTime() - 3_600_000);
      const params = new URLSearchParams({
        name: "http_requests",
        start: start.toISOString(),
        end: end.toISOString(),
        step: "1m",
        fn: "sum",
      });
      const res = await get<QueryResponse>(`/admin/metrics/query?${params}`);
      // Aggregate all series (method/path/status combos) into total per minute.
      const buckets = new Map<number, number>();
      for (const s of res.series ?? []) {
        for (const p of s.points) {
          buckets.set(p.t, (buckets.get(p.t) ?? 0) + p.v);
        }
      }
      const points: TrafficPoint[] = [...buckets.entries()]
        .sort(([a], [b]) => a - b)
        .map(([t, v]) => ({
          time: new Date(t).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" }),
          Requests: Math.round(v),
        }));
      setTraffic(points);
    } catch {
      // Traffic chart is non-critical — silently fail.
    }
  }, []);

  useEffect(() => {
    loadStats();
    loadActivity();
    loadTraffic();
  }, [loadStats, loadActivity, loadTraffic]);

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

  const successRate = data.http.total_requests > 0
    ? ((data.http.status_2xx + data.http.status_3xx) / data.http.total_requests * 100).toFixed(1)
    : "100.0";

  return (
    <Stack>
      <Title order={3}>Dashboard</Title>

      <SimpleGrid cols={{ base: 1, xs: 2, md: 4 }}>
        <StatCard title="Users" value={data.stats.total_users} icon={IconUsers} color="blue" />
        <StatCard title="Active Sessions" value={data.stats.active_sessions} icon={IconServer} color="green" />
        <StatCard title="Database" value={formatBytes(data.database.size_bytes)} icon={IconDatabase} color="violet" />
        <StatCard title="Uptime" value={data.system.uptime} icon={IconClock} color="orange" />
      </SimpleGrid>

      <SimpleGrid cols={{ base: 1, xs: 2, md: 4 }}>
        <StatCard title="Total Requests" value={formatNumber(data.http.total_requests)} icon={IconWorld} color="cyan" />
        <StatCard title="Success Rate" value={`${successRate}%`} icon={IconShieldCheck} color={Number(successRate) >= 99 ? "green" : "yellow"} />
        <StatCard title="Avg Latency" value={`${data.http.avg_duration_ms.toFixed(1)} ms`} icon={IconClock} color={data.http.avg_duration_ms < 100 ? "teal" : "orange"} />
        <StatCard title="Emails Sent" value={data.email.sent} icon={IconMail} color="grape" />
      </SimpleGrid>

      {traffic.length > 0 && (
        <Card withBorder padding="lg" radius="md">
          <Text size="sm" c="dimmed" mb="sm">HTTP Requests (last hour)</Text>
          <AreaChart
            h={200}
            data={traffic}
            dataKey="time"
            series={[{ name: "Requests", color: "cyan.6" }]}
            curveType="monotone"
            withDots={false}
            withGradient
            gridAxis="x"
            tickLine="none"
            withYAxis={false}
          />
        </Card>
      )}

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
              <InfoRow label="Go Version" value={data.system.go_version} />
              <InfoRow label="Goroutines" value={data.system.goroutines} />
              <InfoRow label="Memory (alloc)" value={`${data.system.memory_alloc_mb.toFixed(1)} MB`} />
              <InfoRow label="Memory (sys)" value={`${data.system.memory_sys_mb.toFixed(1)} MB`} />
            </Stack>
          </Card>
        </Grid.Col>

        <Grid.Col span={{ base: 12, md: 6 }}>
          <Card withBorder padding="lg" radius="md">
            <Text fw={600} mb="sm">Database</Text>
            <Stack gap={4}>
              <InfoRow label="Tables" value={data.database.tables} />
              <InfoRow label="Migrations" value={data.database.migrations} />
              <InfoRow label="DB Size" value={formatBytes(data.database.size_bytes)} />
              <InfoRow label="WAL Size" value={formatBytes(data.database.wal_size_bytes)} />
            </Stack>
          </Card>
        </Grid.Col>

        <Grid.Col span={{ base: 12, md: 6 }}>
          <Card withBorder padding="lg" radius="md">
            <Text fw={600} mb="sm">HTTP</Text>
            <Stack gap={4}>
              <InfoRow label="2xx Responses" value={formatNumber(data.http.status_2xx)} />
              <InfoRow label="4xx Responses" value={formatNumber(data.http.status_4xx)} valueColor={data.http.status_4xx > 0 ? "yellow" : undefined} />
              <InfoRow label="5xx Responses" value={formatNumber(data.http.status_5xx)} valueColor={data.http.status_5xx > 0 ? "red" : undefined} />
              <InfoRow label="Bytes Written" value={formatBytes(data.http.bytes_written)} />
            </Stack>
          </Card>
        </Grid.Col>

        <Grid.Col span={{ base: 12, md: 6 }}>
          <Card withBorder padding="lg" radius="md">
            <Text fw={600} mb="sm">Job Queue</Text>
            <Stack gap={4}>
              <InfoRow label="Pending" value={data.queue.pending} />
              <InfoRow label="Running" value={data.queue.running} />
              <InfoRow label="Completed" value={data.queue.completed} />
              <InfoRow label="Failed" value={data.queue.failed} valueColor={data.queue.failed > 0 ? "red" : undefined} />
            </Stack>
          </Card>
        </Grid.Col>

        <Grid.Col span={{ base: 12, md: 6 }}>
          <Card withBorder padding="lg" radius="md">
            <Group justify="space-between" mb="sm">
              <Text fw={600}>Auth</Text>
              <ThemeIcon size={24} radius="sm" variant="light" color="cyan">
                <IconShieldCheck size={14} stroke={1.5} />
              </ThemeIcon>
            </Group>
            <Stack gap={4}>
              <InfoRow label="Tokens Issued" value={formatNumber(data.auth.issued)} />
              <InfoRow label="Accepted" value={formatNumber(data.auth.accepted)} />
              <InfoRow label="Rejected" value={formatNumber(data.auth.rejected)} valueColor={data.auth.rejected > 0 ? "orange" : undefined} />
            </Stack>
          </Card>
        </Grid.Col>

        <Grid.Col span={{ base: 12, md: 6 }}>
          <Card withBorder padding="lg" radius="md">
            <Group justify="space-between" mb="sm">
              <Text fw={600}>Webhooks & Email</Text>
              <Group gap={4}>
                <ThemeIcon size={24} radius="sm" variant="light" color="orange">
                  <IconWebhook size={14} stroke={1.5} />
                </ThemeIcon>
                <ThemeIcon size={24} radius="sm" variant="light" color="grape">
                  <IconMail size={14} stroke={1.5} />
                </ThemeIcon>
              </Group>
            </Group>
            <Stack gap={4}>
              <InfoRow label="Webhook Sends" value={data.webhook.sends} />
              <InfoRow label="Webhook Failures" value={data.webhook.failures} valueColor={data.webhook.failures > 0 ? "red" : undefined} />
              <InfoRow label="Emails Sent" value={data.email.sent} />
              <InfoRow label="Email Errors" value={data.email.errors} valueColor={data.email.errors > 0 ? "red" : undefined} />
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
