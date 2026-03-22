import { useEffect, useState } from "react";
import {
  ActionIcon,
  Alert,
  Button,
  Card,
  Group,
  Loader,
  Stack,
  Text,
  TextInput,
  Title,
} from "@mantine/core";
import { notifications } from "@mantine/notifications";
import {
  IconAlertCircle,
  IconCheck,
  IconRefresh,
  IconX,
} from "@tabler/icons-react";
import { get, put } from "@/lib/api";

interface Setting {
  key: string;
  value: string;
  group_name: string;
  updated_at: string;
}

export default function SettingsPage() {
  const [settings, setSettings] = useState<Setting[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [editingKey, setEditingKey] = useState<string | null>(null);
  const [editValue, setEditValue] = useState("");
  const [saving, setSaving] = useState(false);

  async function load() {
    try {
      const data = await get<{ settings: Setting[] }>("/admin/settings");
      setSettings(data.settings);
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load");
    }
  }

  useEffect(() => {
    load();
  }, []);

  function startEdit(s: Setting) {
    setEditingKey(s.key);
    setEditValue(s.value);
  }

  function cancelEdit() {
    setEditingKey(null);
    setEditValue("");
  }

  async function saveEdit(key: string) {
    setSaving(true);
    try {
      const updated = await put<Setting>(`/admin/settings/${key}`, { value: editValue });
      setSettings((prev) => prev.map((s) => (s.key === key ? updated : s)));
      setEditingKey(null);
      setEditValue("");
      notifications.show({ message: `Setting "${key}" updated`, color: "green", icon: <IconCheck size={16} /> });
    } catch (err) {
      notifications.show({
        message: err instanceof Error ? err.message : "Failed to save",
        color: "red",
        icon: <IconAlertCircle size={16} />,
      });
    } finally {
      setSaving(false);
    }
  }

  // Group settings by group_name
  const grouped: Record<string, Setting[]> = {};
  for (const s of settings) {
    const list = grouped[s.group_name] ?? (grouped[s.group_name] = []);
    list.push(s);
  }

  const groupOrder = Object.keys(grouped).sort((a, b) => {
    if (a === "general") return -1;
    if (b === "general") return 1;
    return a.localeCompare(b);
  });

  return (
    <Stack p="md" gap="md">
      <Group justify="space-between">
        <div>
          <Title order={3}>Settings</Title>
          <Text size="sm" c="dimmed">Application settings stored in the database</Text>
        </div>
        <Button variant="default" size="sm" leftSection={<IconRefresh size={16} />} onClick={load}>
          Refresh
        </Button>
      </Group>

      {error && (
        <Alert icon={<IconAlertCircle size={16} />} color="red" withCloseButton onClose={() => setError(null)}>
          {error}
        </Alert>
      )}

      {settings.length === 0 && !error && <Stack align="center" pt="xl"><Loader /></Stack>}

      {groupOrder.map((group) => (
        <Card key={group} withBorder padding={0}>
          <Group px="md" py="sm" style={{ borderBottom: "1px solid var(--mantine-color-default-border)" }}>
            <Text size="sm" fw={500} c="dimmed" tt="uppercase">{group}</Text>
          </Group>
          <Stack gap={0}>
            {(grouped[group] ?? []).map((s) => (
              <Group
                key={s.key}
                justify="space-between"
                px="md"
                py="sm"
                wrap="nowrap"
                style={{ borderBottom: "1px solid var(--mantine-color-default-border)" }}
              >
                <div style={{ minWidth: 0, flex: 1 }}>
                  <Text size="sm" fw={500} ff="monospace">{s.key}</Text>
                  <Text size="xs" c="dimmed" mt={2}>
                    Updated {new Date(s.updated_at).toLocaleString()}
                  </Text>
                </div>
                <Group gap="xs" wrap="nowrap">
                  {editingKey === s.key ? (
                    <>
                      <TextInput
                        value={editValue}
                        onChange={(e) => setEditValue(e.currentTarget.value)}
                        onKeyDown={(e) => {
                          if (e.key === "Enter") saveEdit(s.key);
                          if (e.key === "Escape") cancelEdit();
                        }}
                        autoFocus
                        size="xs"
                        w={200}
                      />
                      <ActionIcon
                        variant="subtle"
                        size="sm"
                        onClick={() => saveEdit(s.key)}
                        loading={saving}
                        color="green"
                      >
                        <IconCheck size={14} />
                      </ActionIcon>
                      <ActionIcon
                        variant="subtle"
                        size="sm"
                        onClick={cancelEdit}
                        disabled={saving}
                      >
                        <IconX size={14} />
                      </ActionIcon>
                    </>
                  ) : (
                    <Text
                      size="sm"
                      ff="monospace"
                      style={{ cursor: "pointer", padding: "4px 8px", borderRadius: 4 }}
                      className="hover-highlight"
                      onClick={() => startEdit(s)}
                    >
                      {s.value}
                    </Text>
                  )}
                </Group>
              </Group>
            ))}
          </Stack>
        </Card>
      ))}
    </Stack>
  );
}
