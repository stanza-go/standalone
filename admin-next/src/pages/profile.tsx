import { useCallback, useEffect, useState } from "react";
import {
  ActionIcon,
  Alert,
  Badge,
  Button,
  Card,
  Group,
  Loader,
  PasswordInput,
  Stack,
  Text,
  TextInput,
  Title,
} from "@mantine/core";
import { notifications } from "@mantine/notifications";
import {
  IconAlertCircle,
  IconCheck,
  IconDeviceDesktop,
  IconTrash,
} from "@tabler/icons-react";
import { get, put, del, ApiError } from "@/lib/api";

interface AdminProfile {
  id: number;
  email: string;
  name: string;
  role: string;
  scopes: string[];
  created_at: string;
  updated_at: string;
}

interface Session {
  id: string;
  created_at: string;
  expires_at: string;
  current: boolean;
}

function formatRelative(iso: string): string {
  const diff = Date.now() - new Date(iso).getTime();
  const mins = Math.floor(diff / 60000);
  if (mins < 1) return "just now";
  if (mins < 60) return `${mins}m ago`;
  const hours = Math.floor(mins / 60);
  if (hours < 24) return `${hours}h ago`;
  const days = Math.floor(hours / 24);
  return `${days}d ago`;
}

export default function ProfilePage() {
  const [profile, setProfile] = useState<AdminProfile | null>(null);
  const [sessions, setSessions] = useState<Session[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  // Profile form
  const [name, setName] = useState("");
  const [savingProfile, setSavingProfile] = useState(false);

  // Password form
  const [currentPassword, setCurrentPassword] = useState("");
  const [newPassword, setNewPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [savingPassword, setSavingPassword] = useState(false);
  const [passwordError, setPasswordError] = useState<string | null>(null);

  const loadProfile = useCallback(async () => {
    try {
      const data = await get<{ admin: AdminProfile }>("/admin/profile");
      setProfile(data.admin);
      setName(data.admin.name);
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load profile");
    }
  }, []);

  const loadSessions = useCallback(async () => {
    try {
      const data = await get<{ sessions: Session[] }>("/admin/profile/sessions");
      setSessions(data.sessions);
    } catch {
      // Non-critical
    }
  }, []);

  useEffect(() => {
    Promise.all([loadProfile(), loadSessions()]).finally(() => setLoading(false));
  }, [loadProfile, loadSessions]);

  async function saveProfile() {
    setSavingProfile(true);
    try {
      const data = await put<{ admin: AdminProfile }>("/admin/profile", { name });
      setProfile(data.admin);
      setName(data.admin.name);
      notifications.show({ message: "Profile updated", color: "green", icon: <IconCheck size={16} /> });
    } catch (err) {
      if (err instanceof ApiError) {
        notifications.show({ message: err.message, color: "red", icon: <IconAlertCircle size={16} /> });
      }
    } finally {
      setSavingProfile(false);
    }
  }

  async function changePassword() {
    setPasswordError(null);

    if (newPassword !== confirmPassword) {
      setPasswordError("New passwords do not match");
      return;
    }

    setSavingPassword(true);
    try {
      await put("/admin/profile/password", {
        current_password: currentPassword,
        new_password: newPassword,
      });
      setCurrentPassword("");
      setNewPassword("");
      setConfirmPassword("");
      notifications.show({ message: "Password changed. Other sessions have been revoked.", color: "green", icon: <IconCheck size={16} /> });
      loadSessions();
    } catch (err) {
      if (err instanceof ApiError) {
        setPasswordError(err.fields?.current_password || err.fields?.new_password || err.message);
      }
    } finally {
      setSavingPassword(false);
    }
  }

  async function revokeSession(id: string) {
    try {
      await del(`/admin/profile/sessions/${id}`);
      setSessions((prev) => prev.filter((s) => s.id !== id));
      notifications.show({ message: "Session revoked", color: "green", icon: <IconCheck size={16} /> });
    } catch (err) {
      notifications.show({
        message: err instanceof Error ? err.message : "Failed to revoke session",
        color: "red",
        icon: <IconAlertCircle size={16} />,
      });
    }
  }

  if (loading) {
    return <Stack align="center" pt="xl"><Loader /></Stack>;
  }

  return (
    <Stack p="md" gap="md" maw={640}>
      <div>
        <Title order={3}>Profile</Title>
        <Text size="sm" c="dimmed">Manage your account settings and sessions</Text>
      </div>

      {error && (
        <Alert icon={<IconAlertCircle size={16} />} color="red" withCloseButton onClose={() => setError(null)}>
          {error}
        </Alert>
      )}

      {/* Account Information */}
      <Card withBorder padding="lg">
        <Text fw={600} mb="md">Account Information</Text>
        <Stack gap="sm">
          <TextInput
            label="Email"
            value={profile?.email ?? ""}
            disabled
            description="Email cannot be changed"
          />
          <TextInput
            label="Display Name"
            value={name}
            onChange={(e) => setName(e.currentTarget.value)}
            placeholder="Your name"
          />
          <Group grow>
            <div>
              <Text size="sm" fw={500} mb={4}>Role</Text>
              <Text size="sm" tt="capitalize">{profile?.role}</Text>
            </div>
            <div>
              <Text size="sm" fw={500} mb={4}>Member Since</Text>
              <Text size="sm">
                {profile?.created_at ? new Date(profile.created_at).toLocaleDateString() : "\u2014"}
              </Text>
            </div>
          </Group>
          {profile?.scopes && profile.scopes.length > 0 && (
            <div>
              <Text size="sm" fw={500} mb={4}>Scopes</Text>
              <Group gap="xs">
                {profile.scopes.map((scope) => (
                  <Badge key={scope} variant="light" size="sm">{scope}</Badge>
                ))}
              </Group>
            </div>
          )}
          <Group justify="flex-end" pt="xs">
            <Button
              size="sm"
              onClick={saveProfile}
              disabled={savingProfile || name === profile?.name}
              loading={savingProfile}
            >
              Save Changes
            </Button>
          </Group>
        </Stack>
      </Card>

      {/* Change Password */}
      <Card withBorder padding="lg">
        <Text fw={600} mb="md">Change Password</Text>
        <Stack gap="sm">
          {passwordError && (
            <Alert icon={<IconAlertCircle size={16} />} color="red" variant="light">
              {passwordError}
            </Alert>
          )}
          <PasswordInput
            label="Current Password"
            value={currentPassword}
            onChange={(e) => setCurrentPassword(e.currentTarget.value)}
            placeholder="Enter current password"
          />
          <PasswordInput
            label="New Password"
            value={newPassword}
            onChange={(e) => setNewPassword(e.currentTarget.value)}
            placeholder="At least 8 characters"
          />
          <PasswordInput
            label="Confirm New Password"
            value={confirmPassword}
            onChange={(e) => setConfirmPassword(e.currentTarget.value)}
            placeholder="Confirm new password"
          />
          <Text size="xs" c="dimmed">
            Changing your password will revoke all other active sessions.
          </Text>
          <Group justify="flex-end" pt="xs">
            <Button
              size="sm"
              onClick={changePassword}
              disabled={savingPassword || !currentPassword || !newPassword || !confirmPassword}
              loading={savingPassword}
            >
              Change Password
            </Button>
          </Group>
        </Stack>
      </Card>

      {/* Active Sessions */}
      <Card withBorder padding={0}>
        <Group px="lg" py="sm" style={{ borderBottom: "1px solid var(--mantine-color-default-border)" }}>
          <Text fw={600}>Active Sessions</Text>
          <Badge variant="light" size="sm">{sessions.length}</Badge>
        </Group>
        {sessions.length === 0 ? (
          <Text ta="center" c="dimmed" py="xl" size="sm">No active sessions</Text>
        ) : (
          <Stack gap={0}>
            {sessions.map((session) => (
              <Group
                key={session.id}
                justify="space-between"
                px="lg"
                py="sm"
                wrap="nowrap"
                style={{ borderBottom: "1px solid var(--mantine-color-default-border)" }}
              >
                <Group gap="sm" wrap="nowrap" style={{ minWidth: 0 }}>
                  <IconDeviceDesktop size={16} color="var(--mantine-color-dimmed)" style={{ flexShrink: 0 }} />
                  <div style={{ minWidth: 0 }}>
                    <Group gap="xs">
                      <Text size="sm">Session</Text>
                      {session.current && (
                        <Badge color="green" variant="light" size="xs">Current</Badge>
                      )}
                    </Group>
                    <Text size="xs" c="dimmed">
                      Created {formatRelative(session.created_at)} · Expires {formatRelative(session.expires_at)}
                    </Text>
                  </div>
                </Group>
                {!session.current && (
                  <ActionIcon
                    variant="subtle"
                    color="red"
                    size="sm"
                    onClick={() => revokeSession(session.id)}
                  >
                    <IconTrash size={14} />
                  </ActionIcon>
                )}
              </Group>
            ))}
          </Stack>
        )}
      </Card>
    </Stack>
  );
}
