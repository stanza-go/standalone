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
  Image,
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
  IconFile,
  IconFileText,
  IconHistory,
  IconPhoto,
  IconTrash,
  IconUser,
  IconVideo,
  IconDeviceDesktop,
  IconUpload,
} from "@tabler/icons-react";
import { get, del } from "@/lib/api";

interface UserDetail {
  id: number;
  email: string;
  name: string;
  is_active: boolean;
  created_at: string;
  updated_at: string;
}

interface ActivityEntry {
  id: number;
  admin_id: string;
  admin_email: string;
  admin_name: string;
  action: string;
  details: string;
  ip_address: string;
  created_at: string;
}

interface Session {
  id: string;
  created_at: string;
  expires_at: string;
}

interface UploadEntry {
  id: number;
  uuid: string;
  original_name: string;
  content_type: string;
  size_bytes: number;
  has_thumbnail: boolean;
  created_at: string;
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

function formatSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1048576) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / 1048576).toFixed(1)} MB`;
}

function fileIcon(contentType: string) {
  if (contentType.startsWith("image/")) return <IconPhoto size={16} color="var(--mantine-color-blue-5)" />;
  if (contentType.startsWith("video/")) return <IconVideo size={16} color="var(--mantine-color-violet-5)" />;
  if (contentType === "application/pdf") return <IconFileText size={16} color="var(--mantine-color-red-5)" />;
  return <IconFile size={16} color="var(--mantine-color-dimmed)" />;
}

function actionLabel(action: string): string {
  const map: Record<string, string> = {
    "user.create": "Created",
    "user.update": "Updated",
    "user.delete": "Deleted",
    "user.impersonate": "Impersonated",
  };
  return map[action] || action;
}

export default function UserDetailPage() {
  const { id } = useParams<{ id: string }>();
  const [user, setUser] = useState<UserDetail | null>(null);
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

  // Uploads
  const [uploads, setUploads] = useState<UploadEntry[]>([]);
  const [uploadsTotal, setUploadsTotal] = useState(0);
  const [uploadsPage, setUploadsPage] = useState(1);
  const [uploadsLoading, setUploadsLoading] = useState(false);

  const loadUser = useCallback(async () => {
    try {
      const data = await get<{ user: UserDetail; active_sessions: number }>(
        `/admin/users/${id}`
      );
      setUser(data.user);
      setSessionCount(data.active_sessions);
      setError("");
    } catch (e: any) {
      setError(e.message || "Failed to load user");
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
        `/admin/users/${id}/activity?${params}`
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
        `/admin/users/${id}/sessions`
      );
      setSessions(data.sessions);
      setSessionCount(data.total);
    } catch {
      // Non-critical
    } finally {
      setSessionsLoading(false);
    }
  }, [id]);

  const loadUploads = useCallback(async () => {
    setUploadsLoading(true);
    try {
      const params = new URLSearchParams();
      params.set("limit", String(PAGE_SIZE));
      params.set("offset", String((uploadsPage - 1) * PAGE_SIZE));
      const data = await get<{ uploads: UploadEntry[]; total: number }>(
        `/admin/users/${id}/uploads?${params}`
      );
      setUploads(data.uploads);
      setUploadsTotal(data.total);
    } catch {
      // Non-critical
    } finally {
      setUploadsLoading(false);
    }
  }, [id, uploadsPage]);

  useEffect(() => {
    loadUser();
  }, [loadUser]);

  useEffect(() => {
    if (tab === "activity") loadActivity();
  }, [tab, loadActivity]);

  useEffect(() => {
    if (tab === "sessions") loadSessions();
  }, [tab, loadSessions]);

  useEffect(() => {
    if (tab === "uploads") loadUploads();
  }, [tab, loadUploads]);

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

  if (error || !user) {
    return (
      <Stack p="md">
        <Breadcrumbs>
          <Anchor component={Link} to="/users" size="sm">Users</Anchor>
          <Text size="sm">Not Found</Text>
        </Breadcrumbs>
        <Alert icon={<IconAlertCircle size={16} />} color="red" title="Error">
          {error || "User not found"}
        </Alert>
      </Stack>
    );
  }

  const activityPages = Math.ceil(activityTotal / PAGE_SIZE);
  const uploadsPages = Math.ceil(uploadsTotal / PAGE_SIZE);

  return (
    <Stack p="md" gap="md">
      <Breadcrumbs>
        <Anchor component={Link} to="/users" size="sm">Users</Anchor>
        <Text size="sm">{user.email}</Text>
      </Breadcrumbs>

      <Title order={3}>User Detail</Title>

      {/* Profile card */}
      <Card withBorder padding="lg">
        <Group gap="xs" mb="md">
          <IconUser size={20} />
          <Text fw={600}>Profile</Text>
        </Group>
        <SimpleGrid cols={{ base: 1, sm: 2, lg: 3 }} spacing="md">
          <div>
            <Text size="xs" c="dimmed">Email</Text>
            <Text size="sm" fw={500}>{user.email}</Text>
          </div>
          <div>
            <Text size="xs" c="dimmed">Name</Text>
            <Text size="sm" fw={500}>{user.name || "\u2014"}</Text>
          </div>
          <div>
            <Text size="xs" c="dimmed">Status</Text>
            <Badge color={user.is_active ? "green" : "red"} variant="light" size="sm">
              {user.is_active ? "Active" : "Inactive"}
            </Badge>
          </div>
          <div>
            <Text size="xs" c="dimmed">Created</Text>
            <Text size="sm">{formatTime(user.created_at)}</Text>
          </div>
          <div>
            <Text size="xs" c="dimmed">Updated</Text>
            <Text size="sm">{formatTime(user.updated_at)}</Text>
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
          <Tabs.Tab value="uploads" leftSection={<IconUpload size={16} />}>
            Uploads {uploadsTotal > 0 && <Badge size="xs" variant="light" ml={4}>{uploadsTotal}</Badge>}
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
                      <Table.Th>By</Table.Th>
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
                            <Text size="xs">{e.admin_email || e.admin_id}</Text>
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

        {/* Uploads Tab */}
        <Tabs.Panel value="uploads" pt="md">
          {uploadsLoading ? (
            <Stack align="center" py="xl"><Loader size="sm" /></Stack>
          ) : (
            <Stack gap="sm">
              <Table.ScrollContainer minWidth={500}>
                <Table striped highlightOnHover>
                  <Table.Thead>
                    <Table.Tr>
                      <Table.Th w={40}></Table.Th>
                      <Table.Th>Name</Table.Th>
                      <Table.Th>Type</Table.Th>
                      <Table.Th>Size</Table.Th>
                      <Table.Th>Uploaded</Table.Th>
                    </Table.Tr>
                  </Table.Thead>
                  <Table.Tbody>
                    {uploads.length === 0 ? (
                      <Table.Tr>
                        <Table.Td colSpan={5}>
                          <Text ta="center" c="dimmed" py="lg">No uploads</Text>
                        </Table.Td>
                      </Table.Tr>
                    ) : (
                      uploads.map((u) => (
                        <Table.Tr key={u.id}>
                          <Table.Td>
                            {u.has_thumbnail ? (
                              <Image
                                src={`/api/admin/uploads/${u.id}/thumb`}
                                alt=""
                                w={32}
                                h={32}
                                radius="sm"
                                fit="cover"
                              />
                            ) : (
                              fileIcon(u.content_type)
                            )}
                          </Table.Td>
                          <Table.Td>
                            <Anchor
                              href={`/api/admin/uploads/${u.id}/file`}
                              target="_blank"
                              size="sm"
                            >
                              {u.original_name}
                            </Anchor>
                          </Table.Td>
                          <Table.Td>
                            <Text size="xs" c="dimmed">{u.content_type}</Text>
                          </Table.Td>
                          <Table.Td>
                            <Text size="xs" c="dimmed">{formatSize(u.size_bytes)}</Text>
                          </Table.Td>
                          <Table.Td>
                            <Text size="xs" c="dimmed" title={formatTime(u.created_at)}>
                              {timeAgo(u.created_at)}
                            </Text>
                          </Table.Td>
                        </Table.Tr>
                      ))
                    )}
                  </Table.Tbody>
                </Table>
              </Table.ScrollContainer>
              {uploadsPages > 1 && (
                <Group justify="center">
                  <Pagination total={uploadsPages} value={uploadsPage} onChange={setUploadsPage} size="sm" />
                </Group>
              )}
            </Stack>
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
            Are you sure you want to revoke this session? The user will be logged out immediately.
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
