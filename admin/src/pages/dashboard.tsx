import { useCallback, useEffect, useState } from "react";
import {
  Alert,
  Card,
  Grid,
  Group,
  Loader,
  SimpleGrid,
  Stack,
  Text,
  ThemeIcon,
  Title,
} from "@mantine/core";
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

function StatCard({ title, value, icon: Icon, color }: { title: string; value: string | number; icon: React.FC<{ size?: number; stroke?: number }>; color: string }) {
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

export default function DashboardPage() {
  const [data, setData] = useState<DashboardData | null>(null);
  const [error, setError] = useState("");

  const load = useCallback(async () => {
    try {
      setData(await get<DashboardData>("/admin/dashboard"));
    } catch {
      setError("Failed to load dashboard");
    }
  }, []);

  useEffect(() => {
    load();
  }, [load]);

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

  return (
    <Stack>
      <Title order={3}>Dashboard</Title>

      <SimpleGrid cols={{ base: 1, xs: 2, md: 4 }}>
        <StatCard title="Users" value={data.stats.total_users} icon={IconUsers} color="blue" />
        <StatCard title="Active Sessions" value={data.stats.active_sessions} icon={IconServer} color="green" />
        <StatCard title="Database" value={formatBytes(data.database.size_bytes)} icon={IconDatabase} color="violet" />
        <StatCard title="Uptime" value={data.system.uptime} icon={IconClock} color="orange" />
      </SimpleGrid>

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
    </Stack>
  );
}
