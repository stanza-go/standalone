import { useState, useEffect, useCallback, useRef } from "react";
import { useNavigate } from "react-router";
import {
  ActionIcon,
  Badge,
  Group,
  Indicator,
  Popover,
  ScrollArea,
  Stack,
  Text,
  Tooltip,
  UnstyledButton,
} from "@mantine/core";
import {
  IconBell,
  IconCheck,
  IconChecks,
  IconTrash,
  IconExternalLink,
  IconWifi,
  IconWifiOff,
} from "@tabler/icons-react";
import { notifications as notify } from "@mantine/notifications";
import { get, post, del } from "@/lib/api";

interface Notification {
  id: number;
  type: string;
  title: string;
  message: string;
  read_at?: string;
  created_at: string;
}

interface ListResponse {
  notifications: Notification[];
  total: number;
  unread: number;
}

interface WsEvent {
  type: "notification" | "unread_count";
  notification?: Notification;
  unread_count: number;
}

const TYPE_COLORS: Record<string, string> = {
  info: "blue",
  success: "green",
  warning: "yellow",
  error: "red",
};

function formatRelativeTime(iso: string): string {
  const diff = (Date.now() - new Date(iso).getTime()) / 1000;
  if (diff < 60) return "just now";
  if (diff < 3600) return `${Math.round(diff / 60)}m ago`;
  if (diff < 86400) return `${Math.round(diff / 3600)}h ago`;
  return `${Math.round(diff / 86400)}d ago`;
}

function wsUrl(path: string): string {
  const loc = window.location;
  const proto = loc.protocol === "https:" ? "wss:" : "ws:";
  return `${proto}//${loc.host}/api${path}`;
}

export function NotificationBell() {
  const navigate = useNavigate();
  const [unreadCount, setUnreadCount] = useState(0);
  const [items, setItems] = useState<Notification[]>([]);
  const [opened, setOpened] = useState(false);
  const [loading, setLoading] = useState(false);
  const [wsConnected, setWsConnected] = useState(false);
  const wsRef = useRef<WebSocket | null>(null);
  const reconnectRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const pollRef = useRef<ReturnType<typeof setInterval> | null>(null);

  const fetchCount = useCallback(async () => {
    try {
      const data = await get<{ unread: number }>("/admin/notifications/unread-count");
      setUnreadCount(data.unread);
    } catch {
      // Non-critical.
    }
  }, []);

  const fetchRecent = useCallback(async () => {
    setLoading(true);
    try {
      const data = await get<ListResponse>("/admin/notifications?limit=10&offset=0");
      setItems(data.notifications);
      setUnreadCount(data.unread);
    } catch {
      // Non-critical.
    } finally {
      setLoading(false);
    }
  }, []);

  const stopPolling = useCallback(() => {
    if (pollRef.current) {
      clearInterval(pollRef.current);
      pollRef.current = null;
    }
  }, []);

  const startPolling = useCallback(() => {
    stopPolling();
    pollRef.current = setInterval(() => {
      if (document.visibilityState === "visible") {
        fetchCount();
      }
    }, 30_000);
  }, [fetchCount, stopPolling]);

  const closeWs = useCallback(() => {
    if (reconnectRef.current) {
      clearTimeout(reconnectRef.current);
      reconnectRef.current = null;
    }
    if (wsRef.current) {
      wsRef.current.close();
      wsRef.current = null;
    }
    setWsConnected(false);
  }, []);

  const connectWs = useCallback(() => {
    closeWs();
    const ws = new WebSocket(wsUrl("/admin/notifications/stream"));
    wsRef.current = ws;

    ws.onopen = () => {
      setWsConnected(true);
      stopPolling();
    };

    ws.onmessage = (e) => {
      try {
        const evt: WsEvent = JSON.parse(e.data);
        setUnreadCount(evt.unread_count);

        if (evt.type === "notification" && evt.notification) {
          const n = evt.notification;
          setItems((prev) => [n, ...prev].slice(0, 10));
          notify.show({
            title: n.title,
            message: n.message,
            color: TYPE_COLORS[n.type] || "blue",
            autoClose: 5000,
          });
        }
      } catch {
        // Ignore malformed messages.
      }
    };

    ws.onclose = () => {
      setWsConnected(false);
      wsRef.current = null;
      startPolling();
      reconnectRef.current = setTimeout(() => {
        if (document.visibilityState === "visible") {
          connectWs();
        }
      }, 5_000);
    };

    ws.onerror = () => {
      // onclose fires after this.
    };
  }, [closeWs, stopPolling, startPolling]);

  useEffect(() => {
    fetchCount();
    connectWs();

    function onVisibility() {
      if (document.visibilityState === "visible") {
        if (!wsRef.current || wsRef.current.readyState !== WebSocket.OPEN) {
          connectWs();
        }
      }
    }
    document.addEventListener("visibilitychange", onVisibility);

    return () => {
      document.removeEventListener("visibilitychange", onVisibility);
      closeWs();
      stopPolling();
    };
  }, [fetchCount, connectWs, closeWs, stopPolling]);

  useEffect(() => {
    if (opened) fetchRecent();
  }, [opened, fetchRecent]);

  async function markRead(id: number) {
    try {
      await post(`/admin/notifications/${id}/read`);
      setItems((prev) =>
        prev.map((n) => (n.id === id ? { ...n, read_at: new Date().toISOString() } : n)),
      );
      setUnreadCount((c) => Math.max(0, c - 1));
    } catch {
      // Silently fail.
    }
  }

  async function markAllRead() {
    try {
      await post("/admin/notifications/read-all");
      setItems((prev) =>
        prev.map((n) => ({ ...n, read_at: n.read_at || new Date().toISOString() })),
      );
      setUnreadCount(0);
    } catch {
      // Silently fail.
    }
  }

  async function deleteNotification(id: number) {
    try {
      const deleted = items.find((n) => n.id === id);
      await del(`/admin/notifications/${id}`);
      setItems((prev) => prev.filter((n) => n.id !== id));
      if (deleted && !deleted.read_at) {
        setUnreadCount((c) => Math.max(0, c - 1));
      }
    } catch {
      // Silently fail.
    }
  }

  const badgeLabel = unreadCount > 99 ? "99+" : String(unreadCount);

  return (
    <Popover
      opened={opened}
      onChange={setOpened}
      position="bottom-end"
      width={360}
      shadow="lg"
      withArrow
      arrowSize={10}
    >
      <Popover.Target>
        <Tooltip label={wsConnected ? "Notifications (live)" : "Notifications"}>
          <Indicator
            label={badgeLabel}
            size={18}
            disabled={unreadCount === 0}
            color="red"
            offset={4}
            processing={wsConnected && unreadCount > 0}
          >
            <ActionIcon
              variant="default"
              size="lg"
              onClick={() => setOpened((o) => !o)}
            >
              <IconBell size={18} />
            </ActionIcon>
          </Indicator>
        </Tooltip>
      </Popover.Target>

      <Popover.Dropdown p={0}>
        {/* Header */}
        <Group justify="space-between" px="md" py="sm" style={{ borderBottom: "1px solid var(--mantine-color-default-border)" }}>
          <Group gap={8}>
            <Text fw={600} size="sm">Notifications</Text>
            {wsConnected ? (
              <Tooltip label="Live updates">
                <IconWifi size={14} color="var(--mantine-color-green-6)" />
              </Tooltip>
            ) : (
              <Tooltip label="Polling mode">
                <IconWifiOff size={14} color="var(--mantine-color-dimmed)" />
              </Tooltip>
            )}
            {unreadCount > 0 && (
              <Badge size="sm" variant="filled" color="red" circle>
                {badgeLabel}
              </Badge>
            )}
          </Group>
          {unreadCount > 0 && (
            <Tooltip label="Mark all as read">
              <ActionIcon variant="subtle" size="sm" color="gray" onClick={markAllRead}>
                <IconChecks size={16} />
              </ActionIcon>
            </Tooltip>
          )}
        </Group>

        {/* List */}
        <ScrollArea.Autosize mah={340}>
          {loading && items.length === 0 ? (
            <Text c="dimmed" ta="center" py="xl" size="sm">
              Loading...
            </Text>
          ) : items.length === 0 ? (
            <Text c="dimmed" ta="center" py="xl" size="sm">
              No notifications yet
            </Text>
          ) : (
            <Stack gap={0}>
              {items.map((n) => (
                <UnstyledButton
                  key={n.id}
                  px="md"
                  py="sm"
                  style={{
                    borderBottom: "1px solid var(--mantine-color-default-border)",
                    backgroundColor: n.read_at ? undefined : "var(--mantine-color-default-hover)",
                  }}
                >
                  <Group gap="sm" wrap="nowrap" align="flex-start">
                    {/* Dot indicator */}
                    <div
                      style={{
                        width: 8,
                        height: 8,
                        borderRadius: "50%",
                        marginTop: 5,
                        flexShrink: 0,
                        backgroundColor: n.read_at
                          ? "transparent"
                          : `var(--mantine-color-${TYPE_COLORS[n.type] || "blue"}-6)`,
                      }}
                    />

                    {/* Content */}
                    <div style={{ flex: 1, minWidth: 0 }}>
                      <Text size="sm" fw={500} lineClamp={1}>
                        {n.title}
                      </Text>
                      <Text size="xs" c="dimmed" lineClamp={2} mt={2}>
                        {n.message}
                      </Text>
                      <Text size="xs" c="dimmed" mt={4} style={{ opacity: 0.7 }}>
                        {formatRelativeTime(n.created_at)}
                      </Text>
                    </div>

                    {/* Actions */}
                    <Group gap={4} wrap="nowrap" style={{ flexShrink: 0 }}>
                      {!n.read_at && (
                        <Tooltip label="Mark as read">
                          <ActionIcon
                            variant="subtle"
                            size="xs"
                            color="gray"
                            onClick={(e) => {
                              e.stopPropagation();
                              markRead(n.id);
                            }}
                          >
                            <IconCheck size={14} />
                          </ActionIcon>
                        </Tooltip>
                      )}
                      <Tooltip label="Delete">
                        <ActionIcon
                          variant="subtle"
                          size="xs"
                          color="gray"
                          onClick={(e) => {
                            e.stopPropagation();
                            deleteNotification(n.id);
                          }}
                        >
                          <IconTrash size={14} />
                        </ActionIcon>
                      </Tooltip>
                    </Group>
                  </Group>
                </UnstyledButton>
              ))}
            </Stack>
          )}
        </ScrollArea.Autosize>

        {/* Footer */}
        <UnstyledButton
          w="100%"
          px="md"
          py="xs"
          style={{ borderTop: "1px solid var(--mantine-color-default-border)" }}
          onClick={() => {
            setOpened(false);
            navigate("/notifications");
          }}
        >
          <Group justify="center" gap={6}>
            <Text size="xs" fw={500} c="dimmed">
              View all notifications
            </Text>
            <IconExternalLink size={12} color="var(--mantine-color-dimmed)" />
          </Group>
        </UnstyledButton>
      </Popover.Dropdown>
    </Popover>
  );
}
