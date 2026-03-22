import { useEffect, useState } from "react";
import {
  Alert,
  Button,
  Card,
  Group,
  Loader,
  SimpleGrid,
  Stack,
  Table,
  Text,
  Title,
} from "@mantine/core";
import { notifications } from "@mantine/notifications";
import {
  IconAlertCircle,
  IconArchive,
  IconCheck,
  IconDatabase,
  IconDownload,
  IconLayersSubtract,
  IconRefresh,
  IconServer,
} from "@tabler/icons-react";
import { get, post } from "@/lib/api";

interface DatabaseInfo {
  files: {
    db_size_bytes: number;
    wal_size_bytes: number;
    shm_size_bytes: number;
    path: string;
  };
  pragmas: {
    page_count: number;
    page_size: number;
    freelist_count: number;
    journal_mode: string;
  };
  tables: { name: string; row_count: number }[];
  migrations: { version: number; name: string; applied_at: string }[];
  backups: { name: string; size_bytes: number; created_at: string }[];
}

interface BackupResult {
  name: string;
  size_bytes: number;
  created_at: string;
}

function formatBytes(bytes: number): string {
  if (bytes === 0) return "0 B";
  const units = ["B", "KB", "MB", "GB"];
  const i = Math.floor(Math.log(bytes) / Math.log(1024));
  return `${(bytes / Math.pow(1024, i)).toFixed(1)} ${units[i]}`;
}

function formatDate(iso: string): string {
  return new Date(iso).toLocaleString();
}

export default function DatabasePage() {
  const [info, setInfo] = useState<DatabaseInfo | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [backingUp, setBackingUp] = useState(false);
  const [downloading, setDownloading] = useState(false);

  async function load() {
    try {
      const data = await get<DatabaseInfo>("/admin/database");
      setInfo(data);
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load");
    }
  }

  useEffect(() => {
    load();
  }, []);

  async function triggerBackup() {
    setBackingUp(true);
    try {
      const result = await post<BackupResult>("/admin/database/backup");
      notifications.show({
        message: `Backup created: ${result.name} (${formatBytes(result.size_bytes)})`,
        color: "green",
        icon: <IconCheck size={16} />,
      });
      load();
    } catch (err) {
      notifications.show({
        message: err instanceof Error ? err.message : "Backup failed",
        color: "red",
        icon: <IconAlertCircle size={16} />,
      });
    } finally {
      setBackingUp(false);
    }
  }

  async function downloadDB() {
    setDownloading(true);
    try {
      const res = await fetch("/api/admin/database/download", {
        credentials: "include",
      });
      if (!res.ok) {
        const data = await res.json().catch(() => ({ error: "Download failed" }));
        throw new Error(data.error ?? "Download failed");
      }
      const blob = await res.blob();
      const url = URL.createObjectURL(blob);
      const a = document.createElement("a");
      a.href = url;
      a.download = "database.sqlite";
      document.body.appendChild(a);
      a.click();
      a.remove();
      URL.revokeObjectURL(url);
    } catch (err) {
      notifications.show({
        message: err instanceof Error ? err.message : "Download failed",
        color: "red",
        icon: <IconAlertCircle size={16} />,
      });
    } finally {
      setDownloading(false);
    }
  }

  return (
    <Stack p="md" gap="md">
      <Group justify="space-between">
        <div>
          <Title order={3}>Database</Title>
          <Text size="sm" c="dimmed">SQLite statistics, migrations, and backups</Text>
        </div>
        <Group>
          <Button variant="default" size="sm" leftSection={<IconRefresh size={16} />} onClick={load}>
            Refresh
          </Button>
          <Button variant="default" size="sm" leftSection={<IconDownload size={16} />} onClick={downloadDB} loading={downloading}>
            Download
          </Button>
          <Button size="sm" leftSection={<IconArchive size={16} />} onClick={triggerBackup} loading={backingUp}>
            Backup Now
          </Button>
        </Group>
      </Group>

      {error && (
        <Alert icon={<IconAlertCircle size={16} />} color="red" withCloseButton onClose={() => setError(null)}>
          {error}
        </Alert>
      )}

      {!info && !error && <Stack align="center" pt="xl"><Loader /></Stack>}

      {info && (
        <>
          {/* Storage Stats */}
          <div>
            <Text size="sm" fw={500} c="dimmed" tt="uppercase" mb="sm">Storage</Text>
            <SimpleGrid cols={{ base: 1, sm: 2, lg: 4 }} spacing="md">
              <StatCard title="Database Size" value={formatBytes(info.files.db_size_bytes)} icon={<IconServer size={18} />} />
              <StatCard title="WAL Size" value={formatBytes(info.files.wal_size_bytes)} icon={<IconDatabase size={18} />} />
              <StatCard title="Journal Mode" value={info.pragmas.journal_mode.toUpperCase()} icon={<IconLayersSubtract size={18} />} />
              <StatCard
                title="Page Count"
                value={String(info.pragmas.page_count)}
                description={`${formatBytes(info.pragmas.page_size)} per page \u00b7 ${info.pragmas.freelist_count} free`}
                icon={<IconDatabase size={18} />}
              />
            </SimpleGrid>
          </div>

          {/* Tables */}
          <div>
            <Text size="sm" fw={500} c="dimmed" tt="uppercase" mb="sm">
              Tables ({info.tables.length})
            </Text>
            <Table.ScrollContainer minWidth={300}>
              <Table striped highlightOnHover withTableBorder>
                <Table.Thead>
                  <Table.Tr>
                    <Table.Th>Name</Table.Th>
                    <Table.Th style={{ textAlign: "right" }}>Rows</Table.Th>
                  </Table.Tr>
                </Table.Thead>
                <Table.Tbody>
                  {info.tables.length === 0 ? (
                    <Table.Tr>
                      <Table.Td colSpan={2}>
                        <Text ta="center" c="dimmed" py="md">No tables</Text>
                      </Table.Td>
                    </Table.Tr>
                  ) : (
                    info.tables.map((t) => (
                      <Table.Tr key={t.name}>
                        <Table.Td><Text size="xs" ff="monospace">{t.name}</Text></Table.Td>
                        <Table.Td style={{ textAlign: "right" }}>
                          <Text size="sm">{t.row_count.toLocaleString()}</Text>
                        </Table.Td>
                      </Table.Tr>
                    ))
                  )}
                </Table.Tbody>
              </Table>
            </Table.ScrollContainer>
          </div>

          {/* Migrations */}
          <div>
            <Text size="sm" fw={500} c="dimmed" tt="uppercase" mb="sm">
              Migrations ({info.migrations.length})
            </Text>
            <Table.ScrollContainer minWidth={400}>
              <Table striped highlightOnHover withTableBorder>
                <Table.Thead>
                  <Table.Tr>
                    <Table.Th>Version</Table.Th>
                    <Table.Th>Name</Table.Th>
                    <Table.Th style={{ textAlign: "right" }}>Applied</Table.Th>
                  </Table.Tr>
                </Table.Thead>
                <Table.Tbody>
                  {info.migrations.length === 0 ? (
                    <Table.Tr>
                      <Table.Td colSpan={3}>
                        <Text ta="center" c="dimmed" py="md">No migrations applied</Text>
                      </Table.Td>
                    </Table.Tr>
                  ) : (
                    info.migrations.map((m) => (
                      <Table.Tr key={m.version}>
                        <Table.Td><Text size="xs" ff="monospace">{m.version}</Text></Table.Td>
                        <Table.Td><Text size="sm">{m.name}</Text></Table.Td>
                        <Table.Td style={{ textAlign: "right" }}>
                          <Text size="xs" c="dimmed">{formatDate(m.applied_at)}</Text>
                        </Table.Td>
                      </Table.Tr>
                    ))
                  )}
                </Table.Tbody>
              </Table>
            </Table.ScrollContainer>
          </div>

          {/* Backups */}
          <div>
            <Text size="sm" fw={500} c="dimmed" tt="uppercase" mb="sm">
              Backups ({info.backups.length})
            </Text>
            <Table.ScrollContainer minWidth={400}>
              <Table striped highlightOnHover withTableBorder>
                <Table.Thead>
                  <Table.Tr>
                    <Table.Th>File</Table.Th>
                    <Table.Th style={{ textAlign: "right" }}>Size</Table.Th>
                    <Table.Th style={{ textAlign: "right" }}>Created</Table.Th>
                  </Table.Tr>
                </Table.Thead>
                <Table.Tbody>
                  {info.backups.length === 0 ? (
                    <Table.Tr>
                      <Table.Td colSpan={3}>
                        <Text ta="center" c="dimmed" py="md">No backups yet</Text>
                      </Table.Td>
                    </Table.Tr>
                  ) : (
                    info.backups.map((b) => (
                      <Table.Tr key={b.name}>
                        <Table.Td><Text size="xs" ff="monospace">{b.name}</Text></Table.Td>
                        <Table.Td style={{ textAlign: "right" }}>
                          <Text size="sm">{formatBytes(b.size_bytes)}</Text>
                        </Table.Td>
                        <Table.Td style={{ textAlign: "right" }}>
                          <Text size="xs" c="dimmed">{formatDate(b.created_at)}</Text>
                        </Table.Td>
                      </Table.Tr>
                    ))
                  )}
                </Table.Tbody>
              </Table>
            </Table.ScrollContainer>
          </div>

          {/* DB Path */}
          <Text size="xs" c="dimmed">
            Path: <Text span ff="monospace">{info.files.path}</Text>
          </Text>
        </>
      )}
    </Stack>
  );
}

function StatCard({ title, value, description, icon }: {
  title: string;
  value: string;
  description?: string;
  icon: React.ReactNode;
}) {
  return (
    <Card withBorder padding="md">
      <Group justify="space-between" mb="xs">
        <Text size="sm" fw={500}>{title}</Text>
        <Text c="dimmed">{icon}</Text>
      </Group>
      <Text size="xl" fw={700}>{value}</Text>
      {description && <Text size="xs" c="dimmed" mt={4}>{description}</Text>}
    </Card>
  );
}
