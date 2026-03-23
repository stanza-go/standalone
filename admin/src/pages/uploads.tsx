import { useCallback, useEffect, useState } from "react";
import {
  ActionIcon,
  Alert,
  Badge,
  Box,
  Button,
  Checkbox,
  Group,
  Image,
  Loader,
  Modal,
  Pagination,
  SegmentedControl,
  SimpleGrid,
  Stack,
  Table,
  Text,
  Title,
  Tooltip,
} from "@mantine/core";
import { Dropzone } from "@mantine/dropzone";
import { notifications } from "@mantine/notifications";
import {
  IconAlertCircle,
  IconArrowDown,
  IconArrowUp,
  IconArrowsSort,
  IconCheck,
  IconDownload,
  IconEye,
  IconFile,
  IconFileText,
  IconMovie,
  IconPhoto,
  IconTrash,
  IconUpload,
  IconX,
} from "@tabler/icons-react";
import { get, del, post, upload as uploadFile, downloadCSV } from "@/lib/api";
import { EmptyState } from "@/components/empty-state";
import { useSort } from "@/hooks/use-sort";
import { useSelection } from "@/hooks/use-selection";
import { useTableKeyboard } from "@/hooks/use-table-keyboard";

interface UploadItem {
  id: number;
  uuid: string;
  original_name: string;
  content_type: string;
  size_bytes: number;
  has_thumbnail: boolean;
  uploaded_by: string;
  entity_type: string;
  entity_id: string;
  created_at: string;
  deleted_at: string;
}

const PAGE_SIZE = 20;

const TYPE_FILTERS = [
  { value: "", label: "All" },
  { value: "image/", label: "Images" },
  { value: "video/", label: "Videos" },
  { value: "application/pdf", label: "PDFs" },
];

function fileIcon(contentType: string) {
  if (contentType.startsWith("image/")) return <IconPhoto size={18} color="var(--mantine-color-blue-5)" />;
  if (contentType.startsWith("video/")) return <IconMovie size={18} color="var(--mantine-color-violet-5)" />;
  if (contentType === "application/pdf") return <IconFileText size={18} color="var(--mantine-color-red-5)" />;
  return <IconFile size={18} color="var(--mantine-color-gray-5)" />;
}

function formatBytes(bytes: number): string {
  if (bytes === 0) return "0 B";
  const units = ["B", "KB", "MB", "GB"];
  const i = Math.floor(Math.log(bytes) / Math.log(1024));
  const val = bytes / Math.pow(1024, i);
  return `${val.toFixed(i > 0 ? 1 : 0)} ${units[i]}`;
}

function formatTime(iso: string): string {
  if (!iso) return "\u2014";
  return new Date(iso).toLocaleString();
}

function thumbUrl(id: number): string {
  return `/api/admin/uploads/${id}/thumb`;
}

function fileUrl(id: number): string {
  return `/api/admin/uploads/${id}/file`;
}

function SortIcon({ column, sort }: { column: string; sort: { column: string; direction: string } }) {
  if (sort.column !== column) return <IconArrowsSort size={14} stroke={1.5} />;
  return sort.direction === "asc" ? <IconArrowUp size={14} stroke={1.5} /> : <IconArrowDown size={14} stroke={1.5} />;
}

export default function UploadsPage() {
  const [uploads, setUploads] = useState<UploadItem[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [actionLoading, setActionLoading] = useState(false);

  // Filters
  const [typeFilter, setTypeFilter] = useState("");
  const [includeDeleted, setIncludeDeleted] = useState(false);

  const [sort, toggleSort] = useSort("id", "desc");
  const selection = useSelection();

  // Dialogs
  const [uploadOpen, setUploadOpen] = useState(false);
  const [preview, setPreview] = useState<UploadItem | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<UploadItem | null>(null);
  const [bulkDeleteOpen, setBulkDeleteOpen] = useState(false);

  // Upload state
  const [selectedFiles, setSelectedFiles] = useState<File[]>([]);
  const [uploading, setUploading] = useState(false);

  const load = useCallback(async () => {
    setLoading(true);
    setError("");
    try {
      const offset = (page - 1) * PAGE_SIZE;
      const params = new URLSearchParams({
        limit: String(PAGE_SIZE),
        offset: String(offset),
        sort: sort.column,
        order: sort.direction.toUpperCase(),
      });
      if (typeFilter) params.set("content_type", typeFilter);
      if (includeDeleted) params.set("include_deleted", "true");

      const data = await get<{ uploads: UploadItem[]; total: number }>(`/admin/uploads?${params}`);
      setUploads(data.uploads ?? []);
      setTotal(data.total);
    } catch {
      setError("Failed to load uploads");
    } finally {
      setLoading(false);
    }
  }, [page, typeFilter, includeDeleted, sort]);

  useEffect(() => {
    load();
  }, [load]);

  useEffect(() => {
    setPage(1);
    selection.clear();
  }, [typeFilter, includeDeleted]); // eslint-disable-line react-hooks/exhaustive-deps

  useEffect(() => {
    selection.clear();
  }, [page, sort]); // eslint-disable-line react-hooks/exhaustive-deps

  async function handleUpload() {
    if (selectedFiles.length === 0) return;
    setUploading(true);
    try {
      for (const file of selectedFiles) {
        await uploadFile("/admin/uploads", file);
      }
      const count = selectedFiles.length;
      notifications.show({ message: `${count} file(s) uploaded`, color: "green", icon: <IconCheck size={16} /> });
      setUploadOpen(false);
      setSelectedFiles([]);
      setPage(1);
      load();
    } catch {
      notifications.show({ message: "Upload failed", color: "red" });
    } finally {
      setUploading(false);
    }
  }

  async function handleDelete() {
    if (!deleteTarget) return;
    setActionLoading(true);
    try {
      await del(`/admin/uploads/${deleteTarget.id}`);
      notifications.show({ message: "Upload deleted", color: "green", icon: <IconCheck size={16} /> });
      setDeleteTarget(null);
      load();
    } catch {
      notifications.show({ message: "Failed to delete upload", color: "red" });
    } finally {
      setActionLoading(false);
    }
  }

  async function handleBulkDelete() {
    setActionLoading(true);
    try {
      const data = await post<{ affected: number }>("/admin/uploads/bulk-delete", { ids: selection.ids });
      notifications.show({ message: `${data.affected} upload(s) deleted`, color: "green", icon: <IconCheck size={16} /> });
      setBulkDeleteOpen(false);
      selection.clear();
      load();
    } catch {
      notifications.show({ message: "Failed to delete uploads", color: "red" });
    } finally {
      setActionLoading(false);
    }
  }

  async function handleExport() {
    try {
      const params = new URLSearchParams({
        sort: sort.column,
        order: sort.direction.toUpperCase(),
      });
      if (typeFilter) params.set("content_type", typeFilter);
      if (includeDeleted) params.set("include_deleted", "true");
      await downloadCSV(`/admin/uploads/export?${params}`);
    } catch {
      notifications.show({ message: "Failed to export", color: "red" });
    }
  }

  const totalPages = Math.ceil(total / PAGE_SIZE);
  const uploadIds = uploads.map((u) => u.id);

  const tableKeyboard = useTableKeyboard({
    rowCount: uploads.length,
    onSelect: (i) => { const u = uploads[i]; if (u) selection.toggle(u.id); },
  });

  return (
    <Stack>
      {/* Header */}
      <Group justify="space-between" wrap="wrap">
        <Group gap="xs">
          <Title order={3}>Uploads</Title>
          {!loading && <Badge variant="light" size="lg">{total}</Badge>}
        </Group>
        <Group gap="xs">
          <Button variant="subtle" size="xs" leftSection={<IconDownload size={16} />} onClick={handleExport}>
            Export CSV
          </Button>
          <Button leftSection={<IconUpload size={16} />} onClick={() => setUploadOpen(true)}>
            Upload File
          </Button>
        </Group>
      </Group>

      {/* Filters */}
      <Group gap="sm">
        <SegmentedControl
          value={typeFilter}
          onChange={setTypeFilter}
          data={TYPE_FILTERS}
          size="sm"
        />
        <Checkbox
          label="Show deleted"
          checked={includeDeleted}
          onChange={(e) => setIncludeDeleted(e.currentTarget.checked)}
        />
      </Group>

      {/* Error */}
      {error && (
        <Alert icon={<IconAlertCircle size={16} />} color="red" variant="light" withCloseButton onClose={() => setError("")}>
          {error}
          <Button variant="subtle" size="xs" ml="sm" onClick={load}>Retry</Button>
        </Alert>
      )}

      {/* Table */}
      {loading && uploads.length === 0 ? (
        <Group justify="center" pt="xl">
          <Loader />
        </Group>
      ) : uploads.length === 0 ? (
        <EmptyState
          icon={<IconUpload size={24} />}
          title={typeFilter ? "No uploads match this filter" : "No files uploaded"}
          description={typeFilter ? "Try a different filter." : "Uploaded files will appear here."}
        />
      ) : (
        <>
          <Table.ScrollContainer minWidth={600}>
            <Table>
              <Table.Thead>
                <Table.Tr>
                  <Table.Th w={40}>
                    <Checkbox
                      checked={selection.isAllSelected(uploadIds)}
                      indeterminate={selection.count > 0 && !selection.isAllSelected(uploadIds)}
                      onChange={() => selection.toggleAll(uploadIds)}
                      aria-label="Select all"
                    />
                  </Table.Th>
                  <Table.Th w={48}></Table.Th>
                  <Table.Th style={{ cursor: "pointer" }} onClick={() => toggleSort("original_name")}>
                    <Group gap={4} wrap="nowrap">Name <SortIcon column="original_name" sort={sort} /></Group>
                  </Table.Th>
                  <Table.Th style={{ cursor: "pointer" }} onClick={() => toggleSort("content_type")}>
                    <Group gap={4} wrap="nowrap">Type <SortIcon column="content_type" sort={sort} /></Group>
                  </Table.Th>
                  <Table.Th style={{ cursor: "pointer" }} onClick={() => toggleSort("size_bytes")}>
                    <Group gap={4} wrap="nowrap">Size <SortIcon column="size_bytes" sort={sort} /></Group>
                  </Table.Th>
                  <Table.Th>Owner</Table.Th>
                  <Table.Th style={{ cursor: "pointer" }} onClick={() => toggleSort("created_at")}>
                    <Group gap={4} wrap="nowrap">Uploaded <SortIcon column="created_at" sort={sort} /></Group>
                  </Table.Th>
                  <Table.Th ta="right">Actions</Table.Th>
                </Table.Tr>
              </Table.Thead>
              <Table.Tbody {...tableKeyboard.tbodyProps}>
                {uploads.map((u, idx) => (
                    <Table.Tr
                      key={u.id}
                      bg={selection.isSelected(u.id) ? "var(--mantine-primary-color-light)" : undefined}
                      style={{ ...(u.deleted_at ? { opacity: 0.5 } : {}), ...(tableKeyboard.isFocused(idx) ? { outline: "2px solid var(--mantine-primary-color-filled)", outlineOffset: -2 } : {}) }}
                    >
                      <Table.Td>
                        <Checkbox
                          checked={selection.isSelected(u.id)}
                          onChange={() => selection.toggle(u.id)}
                          aria-label={`Select ${u.original_name}`}
                        />
                      </Table.Td>
                      <Table.Td>
                        {u.has_thumbnail ? (
                          <Image
                            src={thumbUrl(u.id)}
                            alt=""
                            w={32}
                            h={32}
                            radius="sm"
                            fit="cover"
                            style={{ background: "var(--mantine-color-default-hover)" }}
                          />
                        ) : (
                          <Box w={32} h={32} style={{ display: "flex", alignItems: "center", justifyContent: "center", borderRadius: "var(--mantine-radius-sm)", background: "var(--mantine-color-default-hover)" }}>
                            {fileIcon(u.content_type)}
                          </Box>
                        )}
                      </Table.Td>
                      <Table.Td>
                        <Text
                          size="sm"
                          fw={500}
                          style={{ cursor: "pointer" }}
                          c="blue"
                          truncate
                          maw={200}
                          onClick={() => setPreview(u)}
                        >
                          {u.original_name}
                        </Text>
                        {u.deleted_at && <Badge variant="light" color="red" size="xs">Deleted</Badge>}
                      </Table.Td>
                      <Table.Td>
                        <Text size="xs" c="dimmed">{u.content_type}</Text>
                      </Table.Td>
                      <Table.Td>
                        <Text size="xs" c="dimmed">{formatBytes(u.size_bytes)}</Text>
                      </Table.Td>
                      <Table.Td>
                        <Text size="xs" c="dimmed">
                          {u.entity_type && u.entity_id ? `${u.entity_type}:${u.entity_id}` : "\u2014"}
                        </Text>
                      </Table.Td>
                      <Table.Td>
                        <Text size="xs" c="dimmed">{formatTime(u.created_at)}</Text>
                      </Table.Td>
                      <Table.Td>
                        <Group gap={4} justify="flex-end" wrap="nowrap">
                          <Tooltip label="Preview">
                            <ActionIcon variant="subtle" size="sm" onClick={() => setPreview(u)}>
                              <IconEye size={16} />
                            </ActionIcon>
                          </Tooltip>
                          <Tooltip label="Download">
                            <ActionIcon
                              variant="subtle"
                              size="sm"
                              component="a"
                              href={fileUrl(u.id)}
                              target="_blank"
                            >
                              <IconDownload size={16} />
                            </ActionIcon>
                          </Tooltip>
                          {!u.deleted_at && (
                            <Tooltip label="Delete">
                              <ActionIcon variant="subtle" size="sm" color="red" onClick={() => setDeleteTarget(u)}>
                                <IconTrash size={16} />
                              </ActionIcon>
                            </Tooltip>
                          )}
                        </Group>
                      </Table.Td>
                    </Table.Tr>
                  ))}
              </Table.Tbody>
            </Table>
          </Table.ScrollContainer>

          {/* Pagination */}
          {totalPages > 1 && (
            <Group justify="space-between">
              <Text size="sm" c="dimmed">
                Showing {(page - 1) * PAGE_SIZE + 1}–{Math.min(page * PAGE_SIZE, total)} of {total}
              </Text>
              <Pagination value={page} onChange={setPage} total={totalPages} size="sm" />
            </Group>
          )}
        </>
      )}

      {/* Bulk action bar */}
      {selection.count > 0 && (
        <Box pos="fixed" bottom={20} left="50%" style={{ transform: "translateX(-50%)", zIndex: 100 }}>
          <Group
            gap="sm"
            px="md"
            py="xs"
            style={(theme) => ({
              background: "var(--mantine-color-body)",
              border: "1px solid var(--mantine-color-default-border)",
              borderRadius: theme.radius.md,
              boxShadow: theme.shadows.lg,
            })}
          >
            <Text size="sm" fw={500}>{selection.count} selected</Text>
            <Button
              variant="light"
              color="red"
              size="xs"
              leftSection={<IconTrash size={14} />}
              onClick={() => setBulkDeleteOpen(true)}
            >
              Delete
            </Button>
            <ActionIcon variant="subtle" size="sm" onClick={selection.clear}>
              <IconX size={14} />
            </ActionIcon>
          </Group>
        </Box>
      )}

      {/* Upload dialog */}
      <Modal
        opened={uploadOpen}
        onClose={() => { setUploadOpen(false); setSelectedFiles([]); }}
        title="Upload File"
        size="md"
      >
        <Stack>
          <Dropzone
            onDrop={(files) => setSelectedFiles((prev) => [...prev, ...files])}
            maxSize={50 * 1024 * 1024}
            multiple
          >
            <Group justify="center" gap="xl" mih={120} style={{ pointerEvents: "none" }}>
              <Dropzone.Accept>
                <IconUpload size={40} stroke={1.5} color="var(--mantine-color-blue-6)" />
              </Dropzone.Accept>
              <Dropzone.Reject>
                <IconX size={40} stroke={1.5} color="var(--mantine-color-red-6)" />
              </Dropzone.Reject>
              <Dropzone.Idle>
                <IconUpload size={40} stroke={1.5} color="var(--mantine-color-dimmed)" />
              </Dropzone.Idle>
              <div>
                <Text size="sm" inline>Drag files here or click to browse</Text>
                <Text size="xs" c="dimmed" inline mt={7}>Max 50 MB per file</Text>
              </div>
            </Group>
          </Dropzone>

          {selectedFiles.length > 0 && (
            <Stack gap="xs">
              <Text size="sm" fw={500}>{selectedFiles.length} file(s) selected</Text>
              {selectedFiles.map((f, i) => (
                <Group key={i} justify="space-between" px="xs" py={4} bg="var(--mantine-color-default-hover)" style={{ borderRadius: "var(--mantine-radius-sm)" }}>
                  <Text size="sm" truncate style={{ flex: 1 }}>{f.name}</Text>
                  <Text size="xs" c="dimmed">{formatBytes(f.size)}</Text>
                </Group>
              ))}
            </Stack>
          )}

          <Group justify="flex-end">
            <Button variant="default" onClick={() => { setUploadOpen(false); setSelectedFiles([]); }}>Cancel</Button>
            <Button onClick={handleUpload} loading={uploading} disabled={selectedFiles.length === 0}>
              Upload {selectedFiles.length > 0 ? `${selectedFiles.length} file(s)` : ""}
            </Button>
          </Group>
        </Stack>
      </Modal>

      {/* Preview dialog */}
      <Modal
        opened={!!preview}
        onClose={() => setPreview(null)}
        title={preview?.original_name ?? "Preview"}
        size="lg"
      >
        {preview && (
          <Stack>
            {/* Preview area */}
            {preview.content_type.startsWith("image/") ? (
              <Box bg="var(--mantine-color-default-hover)" p="md" style={{ borderRadius: "var(--mantine-radius-md)", textAlign: "center" }}>
                <Image
                  src={fileUrl(preview.id)}
                  alt={preview.original_name}
                  mah={400}
                  fit="contain"
                  radius="sm"
                />
              </Box>
            ) : preview.content_type.startsWith("video/") ? (
              <Box bg="var(--mantine-color-default-hover)" p="md" style={{ borderRadius: "var(--mantine-radius-md)", textAlign: "center" }}>
                <video
                  src={fileUrl(preview.id)}
                  controls
                  style={{ maxHeight: 400, maxWidth: "100%", borderRadius: "var(--mantine-radius-sm)" }}
                />
              </Box>
            ) : (
              <Box bg="var(--mantine-color-default-hover)" py="xl" style={{ borderRadius: "var(--mantine-radius-md)", textAlign: "center" }}>
                {fileIcon(preview.content_type)}
                <Text size="sm" c="dimmed" mt="xs">No preview available</Text>
              </Box>
            )}

            {/* Metadata */}
            <SimpleGrid cols={2} spacing="xs">
              <div>
                <Text size="xs" c="dimmed">Type</Text>
                <Text size="sm" fw={500}>{preview.content_type}</Text>
              </div>
              <div>
                <Text size="xs" c="dimmed">Size</Text>
                <Text size="sm" fw={500}>{formatBytes(preview.size_bytes)}</Text>
              </div>
              <div>
                <Text size="xs" c="dimmed">Owner</Text>
                <Text size="sm" fw={500}>
                  {preview.entity_type && preview.entity_id ? `${preview.entity_type}:${preview.entity_id}` : "\u2014"}
                </Text>
              </div>
              <div>
                <Text size="xs" c="dimmed">Uploaded</Text>
                <Text size="sm" fw={500}>{formatTime(preview.created_at)}</Text>
              </div>
              <div style={{ gridColumn: "span 2" }}>
                <Text size="xs" c="dimmed">UUID</Text>
                <Text size="xs" ff="monospace" style={{ wordBreak: "break-all" }}>{preview.uuid}</Text>
              </div>
              {preview.deleted_at && (
                <div>
                  <Text size="xs" c="dimmed">Deleted</Text>
                  <Text size="sm" fw={500} c="red">{formatTime(preview.deleted_at)}</Text>
                </div>
              )}
            </SimpleGrid>

            <Group justify="flex-end">
              <Button
                variant="default"
                leftSection={<IconDownload size={16} />}
                component="a"
                href={fileUrl(preview.id)}
                target="_blank"
              >
                Download
              </Button>
              <Button variant="default" onClick={() => setPreview(null)}>Close</Button>
            </Group>
          </Stack>
        )}
      </Modal>

      {/* Delete confirmation */}
      <Modal opened={!!deleteTarget} onClose={() => setDeleteTarget(null)} title="Delete Upload">
        <Stack>
          <Text size="sm">Are you sure you want to delete this file? This action cannot be undone.</Text>
          {deleteTarget && (
            <Box>
              <Text size="sm"><strong>File:</strong> {deleteTarget.original_name}</Text>
              <Text size="sm"><strong>Size:</strong> {formatBytes(deleteTarget.size_bytes)}</Text>
            </Box>
          )}
          <Group justify="flex-end">
            <Button variant="default" onClick={() => setDeleteTarget(null)}>Cancel</Button>
            <Button color="red" onClick={handleDelete} loading={actionLoading}>Delete</Button>
          </Group>
        </Stack>
      </Modal>

      {/* Bulk delete confirmation */}
      <Modal opened={bulkDeleteOpen} onClose={() => setBulkDeleteOpen(false)} title="Delete Uploads">
        <Stack>
          <Text size="sm">
            Are you sure you want to delete <strong>{selection.count}</strong> upload(s)? This action cannot be undone.
          </Text>
          <Group justify="flex-end">
            <Button variant="default" onClick={() => setBulkDeleteOpen(false)}>Cancel</Button>
            <Button color="red" onClick={handleBulkDelete} loading={actionLoading}>
              Delete {selection.count} upload(s)
            </Button>
          </Group>
        </Stack>
      </Modal>
    </Stack>
  );
}
