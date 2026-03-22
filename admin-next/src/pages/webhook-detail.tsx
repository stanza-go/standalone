import { useCallback, useEffect, useState } from "react";
import { useParams, useNavigate, Link } from "react-router";
import {
  Alert,
  Anchor,
  Badge,
  Breadcrumbs,
  Button,
  Card,
  Code,
  CopyButton,
  ActionIcon,
  Group,
  Loader,
  NativeSelect,
  Pagination,
  SimpleGrid,
  Stack,
  Table,
  Text,
  Title,
  Tooltip,
} from "@mantine/core";
import { notifications } from "@mantine/notifications";
import {
  IconAlertCircle,
  IconArrowLeft,
  IconCheck,
  IconChevronDown,
  IconChevronRight,
  IconCopy,
  IconSend,
} from "@tabler/icons-react";
import { get, post } from "@/lib/api";

interface Webhook {
  id: number;
  url: string;
  secret: string;
  description: string;
  events: string[];
  is_active: boolean;
  created_by: number;
  created_at: string;
  updated_at: string;
}

interface Delivery {
  id: number;
  webhook_id: number;
  delivery_id: string;
  event: string;
  payload: string;
  status: string;
  status_code: number;
  response_body: string;
  attempts: number;
  created_at: string;
  completed_at: string;
}

interface WebhookDetailResponse {
  webhook: Webhook;
  total_deliveries: number;
  success_count: number;
  failed_count: number;
}

const PAGE_SIZE = 20;

function formatTime(iso: string): string {
  if (!iso) return "\u2014";
  const d = new Date(iso);
  return d.toLocaleDateString() + " " + d.toLocaleTimeString();
}

function formatJSON(str: string): string {
  try {
    return JSON.stringify(JSON.parse(str), null, 2);
  } catch {
    return str;
  }
}

function deliveryStatusColor(status: string): string {
  if (status === "success") return "green";
  if (status === "failed") return "red";
  return "yellow";
}

export default function WebhookDetailPage() {
  const { id } = useParams();
  const navigate = useNavigate();
  const [detail, setDetail] = useState<WebhookDetailResponse | null>(null);
  const [deliveries, setDeliveries] = useState<Delivery[]>([]);
  const [deliveryTotal, setDeliveryTotal] = useState(0);
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(true);
  const [sending, setSending] = useState(false);
  const [expandedDelivery, setExpandedDelivery] = useState<number | null>(null);

  const [page, setPage] = useState(1);
  const [statusFilter, setStatusFilter] = useState("");

  const loadDetail = useCallback(async () => {
    try {
      const data = await get<WebhookDetailResponse>(`/admin/webhooks/${id}`);
      setDetail(data);
      setError("");
    } catch (e: any) {
      setError(e.message || "Failed to load webhook");
    }
  }, [id]);

  const loadDeliveries = useCallback(async () => {
    try {
      const params = new URLSearchParams();
      params.set("limit", String(PAGE_SIZE));
      params.set("offset", String((page - 1) * PAGE_SIZE));
      if (statusFilter) params.set("status", statusFilter);

      const data = await get<{ deliveries: Delivery[]; total: number }>(
        `/admin/webhooks/${id}/deliveries?${params}`
      );
      setDeliveries(data.deliveries);
      setDeliveryTotal(data.total);
    } catch {
      // Non-blocking
    }
  }, [id, page, statusFilter]);

  useEffect(() => {
    Promise.all([loadDetail(), loadDeliveries()]).finally(() => setLoading(false));
  }, [loadDetail, loadDeliveries]);

  useEffect(() => {
    setPage(1);
  }, [statusFilter]);

  async function sendTest() {
    setSending(true);
    try {
      await post(`/admin/webhooks/${id}/test`);
      notifications.show({ message: "Test event queued for delivery", color: "green", icon: <IconCheck size={16} /> });
      setTimeout(() => {
        loadDeliveries();
        loadDetail();
      }, 2000);
    } catch (e: any) {
      notifications.show({ message: e.message || "Failed to send test event", color: "red", icon: <IconAlertCircle size={16} /> });
    } finally {
      setSending(false);
    }
  }

  if (loading) {
    return <Stack align="center" pt="xl"><Loader /></Stack>;
  }

  if (error || !detail) {
    return (
      <Stack p="md">
        <Alert icon={<IconAlertCircle size={16} />} color="red" title="Error">
          {error || "Webhook not found"}
        </Alert>
      </Stack>
    );
  }

  const wh = detail.webhook;
  const totalPages = Math.ceil(deliveryTotal / PAGE_SIZE);

  return (
    <Stack p="md" gap="md">
      <Breadcrumbs>
        <Anchor component={Link} to="/webhooks" size="sm">Webhooks</Anchor>
        <Text size="sm">{wh.description || wh.url}</Text>
      </Breadcrumbs>

      {/* Header */}
      <Group justify="space-between" align="flex-start">
        <div>
          <Title order={3}>{wh.description || "Webhook Detail"}</Title>
          <Text size="sm" c="dimmed" ff="monospace" mt={4}>{wh.url}</Text>
        </div>
        <Group>
          <Button variant="default" size="sm" leftSection={<IconArrowLeft size={16} />} onClick={() => navigate("/webhooks")}>
            Back
          </Button>
          <Button size="sm" leftSection={<IconSend size={16} />} onClick={sendTest} loading={sending}>
            Send Test
          </Button>
        </Group>
      </Group>

      {/* Stats cards */}
      <SimpleGrid cols={{ base: 2, md: 4 }} spacing="md">
        <Card withBorder padding="md">
          <Text size="sm" c="dimmed">Status</Text>
          <Text size="lg" fw={600} mt={4} c={wh.is_active ? "green" : "yellow"}>
            {wh.is_active ? "Active" : "Paused"}
          </Text>
        </Card>
        <Card withBorder padding="md">
          <Text size="sm" c="dimmed">Total Deliveries</Text>
          <Text size="lg" fw={600} mt={4}>{detail.total_deliveries}</Text>
        </Card>
        <Card withBorder padding="md">
          <Text size="sm" c="dimmed">Successful</Text>
          <Text size="lg" fw={600} mt={4} c="green">{detail.success_count}</Text>
        </Card>
        <Card withBorder padding="md">
          <Text size="sm" c="dimmed">Failed</Text>
          <Text size="lg" fw={600} mt={4} c="red">{detail.failed_count}</Text>
        </Card>
      </SimpleGrid>

      {/* Configuration */}
      <Card withBorder padding="md">
        <Text fw={600} mb="sm">Configuration</Text>
        <SimpleGrid cols={{ base: 1, md: 2 }} spacing="sm">
          <div>
            <Text size="sm" c="dimmed" component="span">Events: </Text>
            <Text size="sm" fw={500} component="span">{wh.events.join(", ")}</Text>
          </div>
          <div>
            <Text size="sm" c="dimmed" component="span">Created: </Text>
            <Text size="sm" fw={500} component="span">{formatTime(wh.created_at)}</Text>
          </div>
        </SimpleGrid>
        <Group mt="sm" gap="xs" align="center">
          <Text size="sm" c="dimmed">Signing Secret:</Text>
          <Code>{wh.secret.slice(0, 12)}...</Code>
          <CopyButton value={wh.secret}>
            {({ copied, copy }) => (
              <Tooltip label={copied ? "Copied" : "Copy"}>
                <ActionIcon variant="subtle" size="sm" onClick={copy} color={copied ? "green" : "gray"}>
                  {copied ? <IconCheck size={14} /> : <IconCopy size={14} />}
                </ActionIcon>
              </Tooltip>
            )}
          </CopyButton>
        </Group>
      </Card>

      {/* Delivery History */}
      <Group justify="space-between" align="center">
        <Text fw={600} size="lg">Delivery History</Text>
        <NativeSelect
          value={statusFilter}
          onChange={(e) => setStatusFilter(e.currentTarget.value)}
          data={[
            { value: "", label: "All statuses" },
            { value: "success", label: "Success" },
            { value: "failed", label: "Failed" },
            { value: "pending", label: "Pending" },
          ]}
          size="xs"
          w={150}
        />
      </Group>

      <Table.ScrollContainer minWidth={600}>
        <Table striped highlightOnHover>
          <Table.Thead>
            <Table.Tr>
              <Table.Th w={30}></Table.Th>
              <Table.Th>Event</Table.Th>
              <Table.Th>Status</Table.Th>
              <Table.Th>HTTP</Table.Th>
              <Table.Th style={{ textAlign: "right" }}>Attempts</Table.Th>
              <Table.Th>Delivery ID</Table.Th>
              <Table.Th>Time</Table.Th>
            </Table.Tr>
          </Table.Thead>
          <Table.Tbody>
            {deliveries.length === 0 ? (
              <Table.Tr>
                <Table.Td colSpan={7}>
                  <Text ta="center" c="dimmed" py="lg">No deliveries yet</Text>
                </Table.Td>
              </Table.Tr>
            ) : (
              deliveries.map((d) => (
                <>
                  <Table.Tr
                    key={d.id}
                    style={{ cursor: "pointer" }}
                    onClick={() => setExpandedDelivery(expandedDelivery === d.id ? null : d.id)}
                  >
                    <Table.Td>
                      {expandedDelivery === d.id
                        ? <IconChevronDown size={14} />
                        : <IconChevronRight size={14} />}
                    </Table.Td>
                    <Table.Td>
                      <Badge color="blue" variant="light" size="sm">{d.event}</Badge>
                    </Table.Td>
                    <Table.Td>
                      <Badge color={deliveryStatusColor(d.status)} variant="light" size="sm">
                        {d.status.charAt(0).toUpperCase() + d.status.slice(1)}
                      </Badge>
                    </Table.Td>
                    <Table.Td>
                      <Text size="xs" ff="monospace" c="dimmed">
                        {d.status_code > 0 ? d.status_code : "\u2014"}
                      </Text>
                    </Table.Td>
                    <Table.Td style={{ textAlign: "right" }}>
                      <Text size="xs">{d.attempts}</Text>
                    </Table.Td>
                    <Table.Td>
                      <Text size="xs" ff="monospace" c="dimmed">
                        {d.delivery_id ? d.delivery_id.slice(0, 16) + "..." : "\u2014"}
                      </Text>
                    </Table.Td>
                    <Table.Td>
                      <Text size="xs" c="dimmed">{formatTime(d.created_at)}</Text>
                    </Table.Td>
                  </Table.Tr>
                  {expandedDelivery === d.id && (
                    <Table.Tr key={`${d.id}-detail`}>
                      <Table.Td colSpan={7} style={{ background: "var(--mantine-color-gray-0)", paddingBlock: 16 }}>
                        <Stack gap="sm" px="sm">
                          <div>
                            <Text size="xs" fw={500} c="dimmed" mb={4}>Request Payload</Text>
                            <Code block style={{ maxHeight: 200, overflow: "auto" }}>
                              {formatJSON(d.payload)}
                            </Code>
                          </div>
                          {d.response_body && (
                            <div>
                              <Text size="xs" fw={500} c="dimmed" mb={4}>Response Body</Text>
                              <Code block style={{ maxHeight: 200, overflow: "auto" }}>
                                {d.response_body.slice(0, 2048)}
                              </Code>
                            </div>
                          )}
                          {d.completed_at && (
                            <Text size="xs" c="dimmed">
                              Completed: {formatTime(d.completed_at)}
                            </Text>
                          )}
                        </Stack>
                      </Table.Td>
                    </Table.Tr>
                  )}
                </>
              ))
            )}
          </Table.Tbody>
        </Table>
      </Table.ScrollContainer>

      {totalPages > 1 && (
        <Group justify="center">
          <Pagination total={totalPages} value={page} onChange={setPage} size="sm" />
        </Group>
      )}
    </Stack>
  );
}
