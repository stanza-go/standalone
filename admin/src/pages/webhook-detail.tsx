import { useCallback, useEffect, useState } from "react";
import { useParams, useNavigate } from "react-router";
import { toast } from "sonner";
import { get, post } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Breadcrumb } from "@/components/ui/breadcrumb";
import { Copy, Check, Send, ArrowLeft } from "lucide-react";
import { Spinner } from "@/components/ui/spinner";
import { ErrorAlert } from "@/components/ui/error-alert";
import { Pagination } from "@/components/ui/pagination";
import { TableEmptyRow } from "@/components/ui/empty-state";

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

interface WebhookDetail {
  webhook: Webhook;
  total_deliveries: number;
  success_count: number;
  failed_count: number;
}

export default function WebhookDetailPage() {
  const { id } = useParams();
  const navigate = useNavigate();
  const [detail, setDetail] = useState<WebhookDetail | null>(null);
  const [deliveries, setDeliveries] = useState<Delivery[]>([]);
  const [deliveryTotal, setDeliveryTotal] = useState(0);
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(true);
  const [sending, setSending] = useState(false);
  const [copied, setCopied] = useState(false);
  const [expandedDelivery, setExpandedDelivery] = useState<number | null>(null);

  // Delivery pagination.
  const [page, setPage] = useState(0);
  const pageSize = 20;

  // Delivery status filter.
  const [statusFilter, setStatusFilter] = useState("");

  const loadDetail = useCallback(async () => {
    try {
      const data = await get<WebhookDetail>(`/admin/webhooks/${id}`);
      setDetail(data);
      setError("");
    } catch (e: any) {
      setError(e.message || "Failed to load webhook");
    }
  }, [id]);

  const loadDeliveries = useCallback(async () => {
    try {
      const params = new URLSearchParams();
      params.set("limit", String(pageSize));
      params.set("offset", String(page * pageSize));
      if (statusFilter) params.set("status", statusFilter);

      const data = await get<{ deliveries: Delivery[]; total: number }>(
        `/admin/webhooks/${id}/deliveries?${params}`
      );
      setDeliveries(data.deliveries);
      setDeliveryTotal(data.total);
    } catch (e: any) {
      // Non-blocking — detail view still works without deliveries.
    }
  }, [id, page, statusFilter]);

  useEffect(() => {
    Promise.all([loadDetail(), loadDeliveries()]).finally(() => setLoading(false));
  }, [loadDetail, loadDeliveries]);

  useEffect(() => {
    setPage(0);
  }, [statusFilter]);

  async function sendTest() {
    setSending(true);
    try {
      await post(`/admin/webhooks/${id}/test`);
      toast.success("Test event queued for delivery");
      // Reload deliveries after a short delay for the queue to process.
      setTimeout(() => {
        loadDeliveries();
        loadDetail();
      }, 2000);
    } catch (e: any) {
      toast.error(e.message || "Failed to send test event");
    } finally {
      setSending(false);
    }
  }

  async function copySecret() {
    if (!detail) return;
    await navigator.clipboard.writeText(detail.webhook.secret);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  }

  function formatTime(iso: string): string {
    if (!iso) return "\u2014";
    const d = new Date(iso);
    return d.toLocaleDateString() + " " + d.toLocaleTimeString();
  }

  const totalPages = Math.ceil(deliveryTotal / pageSize);

  if (loading) {
    return (
      <div className="p-6 flex items-center justify-center min-h-[300px]">
        <Spinner />
      </div>
    );
  }

  if (error || !detail) {
    return (
      <div className="p-6">
        <ErrorAlert message={error || "Webhook not found"} onRetry={loadDetail} />
      </div>
    );
  }

  const wh = detail.webhook;

  return (
    <div className="p-6">
      <Breadcrumb items={[
        { label: "Webhooks", to: "/webhooks" },
        { label: wh.description || wh.url },
      ]} />

      <div className="flex items-center justify-between mb-6 mt-4">
        <div>
          <h1 className="text-2xl font-bold">{wh.description || "Webhook Detail"}</h1>
          <p className="text-sm text-muted-foreground font-mono mt-1">{wh.url}</p>
        </div>
        <div className="flex items-center gap-2">
          <Button variant="outline" size="sm" onClick={() => navigate("/webhooks")}>
            <ArrowLeft className="h-4 w-4 mr-1" />
            Back
          </Button>
          <Button size="sm" onClick={sendTest} disabled={sending}>
            <Send className="h-4 w-4 mr-1" />
            {sending ? "Sending..." : "Send Test"}
          </Button>
        </div>
      </div>

      {/* Stats cards */}
      <div className="grid grid-cols-2 md:grid-cols-4 gap-4 mb-6">
        <div className="border rounded-lg p-4">
          <p className="text-sm text-muted-foreground">Status</p>
          <p className="text-lg font-semibold mt-1">
            {wh.is_active ? (
              <span className="text-green-600 dark:text-green-400">Active</span>
            ) : (
              <span className="text-yellow-600 dark:text-yellow-400">Paused</span>
            )}
          </p>
        </div>
        <div className="border rounded-lg p-4">
          <p className="text-sm text-muted-foreground">Total Deliveries</p>
          <p className="text-lg font-semibold mt-1">{detail.total_deliveries}</p>
        </div>
        <div className="border rounded-lg p-4">
          <p className="text-sm text-muted-foreground">Successful</p>
          <p className="text-lg font-semibold mt-1 text-green-600 dark:text-green-400">{detail.success_count}</p>
        </div>
        <div className="border rounded-lg p-4">
          <p className="text-sm text-muted-foreground">Failed</p>
          <p className="text-lg font-semibold mt-1 text-red-600 dark:text-red-400">{detail.failed_count}</p>
        </div>
      </div>

      {/* Webhook info */}
      <div className="border rounded-lg p-4 mb-6 space-y-3">
        <div className="flex items-center justify-between">
          <h2 className="font-semibold">Configuration</h2>
        </div>

        <div className="grid grid-cols-1 md:grid-cols-2 gap-3 text-sm">
          <div>
            <span className="text-muted-foreground">Events:</span>{" "}
            <span className="font-medium">
              {wh.events.join(", ")}
            </span>
          </div>
          <div>
            <span className="text-muted-foreground">Created:</span>{" "}
            <span className="font-medium">{formatTime(wh.created_at)}</span>
          </div>
        </div>

        <div className="text-sm">
          <span className="text-muted-foreground">Signing Secret:</span>{" "}
          <span className="inline-flex items-center gap-2">
            <code className="font-mono text-xs bg-muted rounded px-2 py-0.5">
              {wh.secret.slice(0, 12)}{"..."}
            </code>
            <Button variant="ghost" size="sm" className="h-6 w-6 p-0" onClick={copySecret}>
              {copied ? (
                <Check className="h-3 w-3 text-green-600 dark:text-green-500" />
              ) : (
                <Copy className="h-3 w-3" />
              )}
            </Button>
          </span>
        </div>
      </div>

      {/* Delivery history */}
      <div className="flex items-center justify-between mb-4">
        <h2 className="text-lg font-semibold">Delivery History</h2>
        <div className="flex items-center gap-2">
          <select
            value={statusFilter}
            onChange={(e) => setStatusFilter(e.target.value)}
            className="text-sm border rounded-md px-2 py-1 bg-background"
          >
            <option value="">All statuses</option>
            <option value="success">Success</option>
            <option value="failed">Failed</option>
            <option value="pending">Pending</option>
          </select>
        </div>
      </div>

      <div className="border rounded-lg overflow-hidden">
        <table className="w-full text-sm">
          <thead>
            <tr className="bg-muted/50 border-b">
              <th className="text-left p-3 font-medium">Event</th>
              <th className="text-left p-3 font-medium">Status</th>
              <th className="text-left p-3 font-medium hidden md:table-cell">HTTP</th>
              <th className="text-right p-3 font-medium hidden md:table-cell">Attempts</th>
              <th className="text-left p-3 font-medium hidden lg:table-cell">Delivery ID</th>
              <th className="text-left p-3 font-medium">Time</th>
            </tr>
          </thead>
          <tbody>
            {deliveries.length === 0 ? (
              <TableEmptyRow colSpan={6} message="No deliveries yet" />
            ) : (
              deliveries.map((d) => (
                <>
                  <tr
                    key={d.id}
                    className="border-b last:border-0 hover:bg-muted/30 cursor-pointer"
                    onClick={() => setExpandedDelivery(expandedDelivery === d.id ? null : d.id)}
                  >
                    <td className="p-3">
                      <span className="inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium bg-blue-100 text-blue-700 dark:bg-blue-500/10 dark:text-blue-400">
                        {d.event}
                      </span>
                    </td>
                    <td className="p-3">
                      <DeliveryStatusBadge status={d.status} />
                    </td>
                    <td className="p-3 font-mono text-xs text-muted-foreground hidden md:table-cell">
                      {d.status_code > 0 ? d.status_code : "\u2014"}
                    </td>
                    <td className="p-3 text-right text-xs hidden md:table-cell">{d.attempts}</td>
                    <td className="p-3 font-mono text-xs text-muted-foreground hidden lg:table-cell">
                      {d.delivery_id ? d.delivery_id.slice(0, 16) + "..." : "\u2014"}
                    </td>
                    <td className="p-3 text-xs text-muted-foreground">
                      {formatTime(d.created_at)}
                    </td>
                  </tr>
                  {expandedDelivery === d.id && (
                    <tr key={`${d.id}-detail`} className="border-b">
                      <td colSpan={6} className="p-4 bg-muted/20">
                        <div className="space-y-3 text-xs">
                          <div>
                            <p className="font-medium text-muted-foreground mb-1">Request Payload</p>
                            <pre className="bg-muted rounded p-3 overflow-x-auto max-h-48">
                              {formatJSON(d.payload)}
                            </pre>
                          </div>
                          {d.response_body && (
                            <div>
                              <p className="font-medium text-muted-foreground mb-1">Response Body</p>
                              <pre className="bg-muted rounded p-3 overflow-x-auto max-h-48">
                                {d.response_body.slice(0, 2048)}
                              </pre>
                            </div>
                          )}
                          {d.completed_at && (
                            <p className="text-muted-foreground">
                              Completed: {formatTime(d.completed_at)}
                            </p>
                          )}
                        </div>
                      </td>
                    </tr>
                  )}
                </>
              ))
            )}
          </tbody>
        </table>
      </div>

      <Pagination
        page={page}
        totalPages={totalPages}
        total={deliveryTotal}
        pageSize={pageSize}
        onPrev={() => setPage(page - 1)}
        onNext={() => setPage(page + 1)}
      />
    </div>
  );
}

function DeliveryStatusBadge({ status }: { status: string }) {
  switch (status) {
    case "success":
      return (
        <span className="inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium bg-green-100 text-green-700 dark:bg-green-500/10 dark:text-green-400">
          Success
        </span>
      );
    case "failed":
      return (
        <span className="inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium bg-red-100 text-red-700 dark:bg-red-500/10 dark:text-red-400">
          Failed
        </span>
      );
    default:
      return (
        <span className="inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium bg-yellow-100 text-yellow-700 dark:bg-yellow-500/10 dark:text-yellow-400">
          Pending
        </span>
      );
  }
}

function formatJSON(str: string): string {
  try {
    return JSON.stringify(JSON.parse(str), null, 2);
  } catch {
    return str;
  }
}
