import { useCallback, useEffect, useState } from "react";
import { useParams, Link } from "react-router";
import {
  ActionIcon,
  Alert,
  Anchor,
  Badge,
  Breadcrumbs,
  Button,
  Card,
  Group,
  Loader,
  Modal,
  Pagination,
  SimpleGrid,
  Stack,
  Table,
  Tabs,
  Text,
  Title,
} from "@mantine/core";
import { notifications } from "@mantine/notifications";
import {
  IconAlertCircle,
  IconCheck,
  IconDeviceDesktop,
  IconHistory,
  IconShield,
  IconTrash,
} from "@tabler/icons-react";
import { get, del } from "@/lib/api";

interface AdminDetail {
  id: number;
  email: string;
  name: string;
  role: string;
  is_active: boolean;
  created_at: string;
  updated_at: string;
}

interface ActivityEntry {
  id: number;
  action: string;
  entity_type: string;
  entity_id: string;
  details: string;
  ip_address: string;
  created_at: string;
}

interface Session {
  id: string;
  created_at: string;
  expires_at: string;
}

const PAGE_SIZE = 20;

function timeAgo(dateStr: string): string {
  if (!dateStr) return "\u2014";
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

function formatTime(iso: string): string {
  if (!iso) return "\u2014";
  const d = new Date(iso);
  return d.toLocaleDateString() + " " + d.toLocaleTimeString();
}

function actionLabel(action: string): string {
  const parts = action.split(".");
  if (parts.length === 2 && parts[0] && parts[1]) {
    const entity = parts[0].charAt(0).toUpperCase() + parts[0].slice(1);
    const verb = parts[1].charAt(0).toUpperCase() + parts[1].slice(1);
    return `${entity} ${verb}`;
  }
  return action;
}

function roleColor(role: string): string {
  if (role === "superadmin") return "violet";
  if (role === "admin") return "blue";
  return "gray";
}

export default function AdminDetailPage() {
  const { id } = useParams<{ id: string }>();
  const [admin, setAdmin] = useState<AdminDetail | null>(null);
  const [sessionCount, setSessionCount] = useState(0);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  const [tab, setTab] = useState<string | null>("activity");

  // Activity
  const [activity, setActivity] = useState<ActivityEntry[]>([]);
  const [activityTotal, setActivityTotal] = useState(0);
  const [activityPage, setActivityPage] = useState(1);
  const [activityLoading, setActivityLoading] = useState(false);

  // Sessions
  const [sessions, setSessions] = useState<Session[]>([]);
  const [sessionsLoading, setSessionsLoading] = useState(false);
  const [revokeTarget, setRevokeTarget] = useState<Session | null>(null);
  const [revoking, setRevoking] = useState(false);

  const loadAdmin = useCallback(async () => {
    try {
      const data = await get<{ admin: AdminDetail; active_sessions: number }>(
        `/admin/admins/${id}`
      );
      setAdmin(data.admin);
      setSessionCount(data.active_sessions);
      setError("");
    } catch (e: any) {
      setError(e.message || "Failed to load admin");
    } finally {
      setLoading(false);
    }
  }, [id]);

  const loadActivity = useCallback(async () => {
    setActivityLoading(true);
    try {
      const params = new URLSearchParams();
      params.set("limit", String(PAGE_SIZE));
      params.set("offset", String((activityPage - 1) * PAGE_SIZE));
      const data = await get<{ entries: ActivityEntry[]; total: number }>(
        `/admin/admins/${id}/activity?${params}`
      );
      setActivity(data.entries);
      setActivityTotal(data.total);
    } catch {
      // Non-critical
    } finally {
      setActivityLoading(false);
    }
  }, [id, activityPage]);

  const loadSessions = useCallback(async () => {
    setSessionsLoading(true);
    try {
      const data = await get<{ sessions: Session[]; total: number }>(
        `/admin/admins/${id}/sessions`
      );
      setSessions(data.sessions);
      setSessionCount(data.total);
    } catch {
      // Non-critical
    } finally {
      setSessionsLoading(false);
    }
  }, [id]);

  useEffect(() => {
    loadAdmin();
  }, [loadAdmin]);

  useEffect(() => {
    if (tab === "activity") loadActivity();
  }, [tab, loadActivity]);

  useEffect(() => {
    if (tab === "sessions") loadSessions();
  }, [tab, loadSessions]);

  async function revokeSession() {
    if (!revokeTarget) return;
    setRevoking(true);
    try {
      await del(`/admin/sessions/${revokeTarget.id}`);
      setRevokeTarget(null);
      notifications.show({ message: "Session revoked", color: "green", icon: <IconCheck size={16} /> });
      loadSessions();
    } catch (e: any) {
      notifications.show({ message: e.message || "Failed to revoke session", color: "red", icon: <IconAlertCircle size={16} /> });
    } finally {
      setRevoking(false);
    }
  }

  if (loading) {
    return <Stack align="center" pt="xl"><Loader /></Stack>;
  }

  if (error || !admin) {
    return (
      <Stack p="md">
        <Breadcrumbs>
          <Anchor component={Link} to="/admins" size="sm">Admin Users</Anchor>
          <Text size="sm">Not Found</Text>
        </Breadcrumbs>
        <Alert icon={<IconAlertCircle size={16} />} color="red" title="Error">
          {error || "Admin not found"}
        </Alert>
      </Stack>
    );
  }

  const activityPages = Math.ceil(activityTotal / PAGE_SIZE);

  return (
    <Stack p="md" gap="md">
      <Breadcrumbs>
        <Anchor component={Link} to="/admins" size="sm">Admin Users</Anchor>
        <Text size="sm">{admin.email}</Text>
      </Breadcrumbs>

      <Title order={3}>Admin Detail</Title>

      {/* Profile card */}
      <Card withBorder padding="lg">
        <Group gap="xs" mb="md">
          <IconShield size={20} />
          <Text fw={600}>Profile</Text>
        </Group>
        <SimpleGrid cols={{ base: 1, sm: 2, lg: 3 }} spacing="md">
          <div>
            <Text size="xs" c="dimmed">Email</Text>
            <Text size="sm" fw={500}>{admin.email}</Text>
          </div>
          <div>
            <Text size="xs" c="dimmed">Name</Text>
            <Text size="sm" fw={500}>{admin.name || "\u2014"}</Text>
          </div>
          <div>
            <Text size="xs" c="dimmed">Role</Text>
            <Badge color={roleColor(admin.role)} variant="light" size="sm">
              {admin.role}
            </Badge>
          </div>
          <div>
            <Text size="xs" c="dimmed">Status</Text>
            <Badge color={admin.is_active ? "green" : "red"} variant="light" size="sm">
              {admin.is_active ? "Active" : "Inactive"}
            </Badge>
          </div>
          <div>
            <Text size="xs" c="dimmed">Created</Text>
            <Text size="sm">{formatTime(admin.created_at)}</Text>
          </div>
          <div>
            <Text size="xs" c="dimmed">Active Sessions</Text>
            <Text size="sm" fw={500}>{sessionCount}</Text>
          </div>
        </SimpleGrid>
      </Card>

      {/* Tabs */}
      <Tabs value={tab} onChange={setTab}>
        <Tabs.List>
          <Tabs.Tab value="activity" leftSection={<IconHistory size={16} />}>
            Activity {activityTotal > 0 && <Badge size="xs" variant="light" ml={4}>{activityTotal}</Badge>}
          </Tabs.Tab>
          <Tabs.Tab value="sessions" leftSection={<IconDeviceDesktop size={16} />}>
            Sessions {sessionCount > 0 && <Badge size="xs" variant="light" ml={4}>{sessionCount}</Badge>}
          </Tabs.Tab>
        </Tabs.List>

        {/* Activity Tab */}
        <Tabs.Panel value="activity" pt="md">
          {activityLoading ? (
            <Stack align="center" py="xl"><Loader size="sm" /></Stack>
          ) : (
            <Stack gap="sm">
              <Table.ScrollContainer minWidth={500}>
                <Table striped highlightOnHover>
                  <Table.Thead>
                    <Table.Tr>
                      <Table.Th>Action</Table.Th>
                      <Table.Th>Target</Table.Th>
                      <Table.Th>Details</Table.Th>
                      <Table.Th>IP</Table.Th>
                      <Table.Th>Time</Table.Th>
                    </Table.Tr>
                  </Table.Thead>
                  <Table.Tbody>
                    {activity.length === 0 ? (
                      <Table.Tr>
                        <Table.Td colSpan={5}>
                          <Text ta="center" c="dimmed" py="lg">No activity recorded</Text>
                        </Table.Td>
                      </Table.Tr>
                    ) : (
                      activity.map((e) => (
                        <Table.Tr key={e.id}>
                          <Table.Td>
                            <Badge variant="light" size="sm">{actionLabel(e.action)}</Badge>
                          </Table.Td>
                          <Table.Td>
                            <Text size="xs" c="dimmed">{e.entity_type} #{e.entity_id}</Text>
                          </Table.Td>
                          <Table.Td>
                            <Text size="xs" c="dimmed" lineClamp={1} maw={200}>
                              {e.details || "\u2014"}
                            </Text>
                          </Table.Td>
                          <Table.Td>
                            <Text size="xs" c="dimmed" ff="monospace">{e.ip_address}</Text>
                          </Table.Td>
                          <Table.Td>
                            <Text size="xs" c="dimmed" title={formatTime(e.created_at)}>
                              {timeAgo(e.created_at)}
                            </Text>
                          </Table.Td>
                        </Table.Tr>
                      ))
                    )}
                  </Table.Tbody>
                </Table>
              </Table.ScrollContainer>
              {activityPages > 1 && (
                <Group justify="center">
                  <Pagination total={activityPages} value={activityPage} onChange={setActivityPage} size="sm" />
                </Group>
              )}
            </Stack>
          )}
        </Tabs.Panel>

        {/* Sessions Tab */}
        <Tabs.Panel value="sessions" pt="md">
          {sessionsLoading ? (
            <Stack align="center" py="xl"><Loader size="sm" /></Stack>
          ) : (
            <Table.ScrollContainer minWidth={400}>
              <Table striped highlightOnHover>
                <Table.Thead>
                  <Table.Tr>
                    <Table.Th>Session ID</Table.Th>
                    <Table.Th>Created</Table.Th>
                    <Table.Th>Expires</Table.Th>
                    <Table.Th style={{ textAlign: "right" }}>Actions</Table.Th>
                  </Table.Tr>
                </Table.Thead>
                <Table.Tbody>
                  {sessions.length === 0 ? (
                    <Table.Tr>
                      <Table.Td colSpan={4}>
                        <Text ta="center" c="dimmed" py="lg">No active sessions</Text>
                      </Table.Td>
                    </Table.Tr>
                  ) : (
                    sessions.map((s) => (
                      <Table.Tr key={s.id}>
                        <Table.Td>
                          <Text size="xs" ff="monospace">{s.id.substring(0, 12)}...</Text>
                        </Table.Td>
                        <Table.Td>
                          <Text size="xs" c="dimmed">{timeAgo(s.created_at)}</Text>
                        </Table.Td>
                        <Table.Td>
                          <Text size="xs" c="dimmed">{formatTime(s.expires_at)}</Text>
                        </Table.Td>
                        <Table.Td style={{ textAlign: "right" }}>
                          <ActionIcon
                            variant="subtle"
                            color="red"
                            size="sm"
                            onClick={() => setRevokeTarget(s)}
                          >
                            <IconTrash size={14} />
                          </ActionIcon>
                        </Table.Td>
                      </Table.Tr>
                    ))
                  )}
                </Table.Tbody>
              </Table>
            </Table.ScrollContainer>
          )}
        </Tabs.Panel>
      </Tabs>

      {/* Revoke Session Modal */}
      <Modal
        opened={!!revokeTarget}
        onClose={() => setRevokeTarget(null)}
        title="Revoke Session"
        size="sm"
      >
        <Stack>
          <Text size="sm">
            Are you sure you want to revoke this session? The admin will be logged out immediately.
          </Text>
          {revokeTarget && (
            <Card withBorder padding="sm">
              <Text size="xs"><Text span fw={500}>Token:</Text> <Text span ff="monospace" size="xs">{revokeTarget.id.substring(0, 16)}...</Text></Text>
              <Text size="xs"><Text span fw={500}>Expires:</Text> {formatTime(revokeTarget.expires_at)}</Text>
            </Card>
          )}
          <Group justify="flex-end">
            <Button variant="default" onClick={() => setRevokeTarget(null)}>Cancel</Button>
            <Button color="red" onClick={revokeSession} loading={revoking}>Revoke</Button>
          </Group>
        </Stack>
      </Modal>
    </Stack>
  );
}
