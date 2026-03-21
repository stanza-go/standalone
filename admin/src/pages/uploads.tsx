import { useCallback, useEffect, useState } from "react";
import { toast } from "sonner";
import { get, del, upload } from "@/lib/api";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogHeader,
  DialogTitle,
  DialogCloseButton,
  DialogBody,
  DialogFooter,
} from "@/components/ui/dialog";
import {
  Trash2,
  Download,
  Image,
  FileText,
  Film,
  File,
  Eye,
  Upload,
} from "lucide-react";
import { TableSkeleton } from "@/components/ui/skeleton";
import { Pagination } from "@/components/ui/pagination";
import { ErrorAlert } from "@/components/ui/error-alert";
import { TableEmptyRow } from "@/components/ui/empty-state";

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

const TYPE_FILTERS = [
  { label: "All", value: "" },
  { label: "Images", value: "image/" },
  { label: "Videos", value: "video/" },
  { label: "PDFs", value: "application/pdf" },
];

function fileIcon(contentType: string) {
  if (contentType.startsWith("image/")) return <Image className="h-4 w-4 text-blue-500" />;
  if (contentType.startsWith("video/")) return <Film className="h-4 w-4 text-purple-500" />;
  if (contentType === "application/pdf") return <FileText className="h-4 w-4 text-red-500" />;
  return <File className="h-4 w-4 text-gray-500" />;
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
  const d = new Date(iso);
  return d.toLocaleDateString() + " " + d.toLocaleTimeString();
}

function thumbUrl(id: number): string {
  return `/api/admin/uploads/${id}/thumb`;
}

function fileUrl(id: number): string {
  return `/api/admin/uploads/${id}/file`;
}

export default function UploadsPage() {
  const [uploads, setUploads] = useState<UploadItem[]>([]);
  const [total, setTotal] = useState(0);
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(true);
  const [acting, setActing] = useState<number | null>(null);

  // Pagination.
  const [page, setPage] = useState(0);
  const pageSize = 20;

  // Filters.
  const [typeFilter, setTypeFilter] = useState("");
  const [includeDeleted, setIncludeDeleted] = useState(false);

  // Preview dialog.
  const [preview, setPreview] = useState<UploadItem | null>(null);

  // Delete confirmation.
  const [deleteId, setDeleteId] = useState<number | null>(null);

  // Upload dialog.
  const [showUpload, setShowUpload] = useState(false);
  const [uploading, setUploading] = useState(false);
  const [dragOver, setDragOver] = useState(false);
  const [selectedFiles, setSelectedFiles] = useState<File[]>([]);
  const [uploadError, setUploadError] = useState("");

  const load = useCallback(async () => {
    try {
      const params = new URLSearchParams();
      params.set("limit", String(pageSize));
      params.set("offset", String(page * pageSize));
      if (typeFilter) params.set("content_type", typeFilter);
      if (includeDeleted) params.set("include_deleted", "true");

      const data = await get<{ uploads: UploadItem[]; total: number }>(
        `/admin/uploads?${params}`
      );
      setUploads(data.uploads ?? []);
      setTotal(data.total);
      setError("");
    } catch (e: unknown) {
      const msg = e instanceof Error ? e.message : "Failed to load uploads";
      setError(msg);
    } finally {
      setLoading(false);
    }
  }, [page, typeFilter, includeDeleted]);

  useEffect(() => {
    load();
  }, [load]);

  async function handleDelete(id: number) {
    setActing(id);
    try {
      await del(`/admin/uploads/${id}`);
      setDeleteId(null);
      toast.success("Upload deleted");
      await load();
    } catch (e: unknown) {
      const msg = e instanceof Error ? e.message : "Failed to delete upload";
      toast.error(msg);
    } finally {
      setActing(null);
    }
  }

  function handleDrop(e: React.DragEvent) {
    e.preventDefault();
    setDragOver(false);
    const files = Array.from(e.dataTransfer.files);
    if (files.length > 0) setSelectedFiles(files);
  }

  function handleFileSelect(e: React.ChangeEvent<HTMLInputElement>) {
    const files = Array.from(e.target.files ?? []);
    if (files.length > 0) setSelectedFiles(files);
  }

  async function handleUpload() {
    if (selectedFiles.length === 0) return;
    setUploading(true);
    setUploadError("");
    try {
      for (const file of selectedFiles) {
        await upload("/admin/uploads", file);
      }
      const count = selectedFiles.length;
      setShowUpload(false);
      setSelectedFiles([]);
      setPage(0);
      toast.success(`${count} file${count !== 1 ? "s" : ""} uploaded`);
      await load();
    } catch (e: unknown) {
      const msg = e instanceof Error ? e.message : "Upload failed";
      setUploadError(msg);
    } finally {
      setUploading(false);
    }
  }

  function closeUploadDialog() {
    setShowUpload(false);
    setSelectedFiles([]);
    setUploadError("");
  }

  function handleFilterChange(value: string) {
    setTypeFilter(value);
    setPage(0);
  }

  const totalPages = Math.ceil(total / pageSize);

  if (loading) {
    return (
      <div className="p-6">
        <h1 className="text-2xl font-bold mb-6">Uploads</h1>
        <TableSkeleton columns={[
          { width: "w-10" },
          { width: "w-32" },
          { width: "w-20", hidden: "hidden md:table-cell" },
          { width: "w-16", hidden: "hidden md:table-cell" },
          { width: "w-20", hidden: "hidden lg:table-cell" },
          { width: "w-24", hidden: "hidden lg:table-cell" },
          { width: "w-20" },
        ]} />
      </div>
    );
  }

  return (
    <div className="p-6">
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-2xl font-bold">Uploads</h1>
          <p className="text-sm text-muted-foreground mt-1">
            {total} file{total !== 1 ? "s" : ""}
          </p>
        </div>
        <Button onClick={() => setShowUpload(true)}>
          <Upload className="h-4 w-4 mr-2" />
          Upload File
        </Button>
      </div>

      {error && (
        <ErrorAlert message={error} onRetry={load} onDismiss={() => setError("")} className="mb-4" />
      )}

      {/* Filters */}
      <div className="mb-4 flex flex-wrap items-center gap-2">
        {TYPE_FILTERS.map((f) => (
          <Button
            key={f.value}
            variant={typeFilter === f.value ? "default" : "outline"}
            size="sm"
            onClick={() => handleFilterChange(f.value)}
          >
            {f.label}
          </Button>
        ))}
        <span className="mx-2 h-4 w-px bg-border" />
        <label className="flex items-center gap-2 text-sm text-muted-foreground cursor-pointer">
          <input
            type="checkbox"
            checked={includeDeleted}
            onChange={(e) => {
              setIncludeDeleted(e.target.checked);
              setPage(0);
            }}
            className="rounded border-border"
          />
          Show deleted
        </label>
      </div>

      <div className="border rounded-lg overflow-hidden">
        <table className="w-full text-sm">
          <thead>
            <tr className="bg-muted/50 border-b">
              <th className="text-left p-3 font-medium w-12"></th>
              <th className="text-left p-3 font-medium">Name</th>
              <th className="text-left p-3 font-medium hidden md:table-cell">Type</th>
              <th className="text-left p-3 font-medium hidden md:table-cell">Size</th>
              <th className="text-left p-3 font-medium hidden lg:table-cell">Owner</th>
              <th className="text-left p-3 font-medium hidden lg:table-cell">Uploaded</th>
              <th className="text-right p-3 font-medium">Actions</th>
            </tr>
          </thead>
          <tbody>
            {uploads.length === 0 ? (
              <TableEmptyRow colSpan={7} message={typeFilter ? "No uploads match this filter" : "No uploads found"} />
            ) : (
              uploads.map((upload) => (
                <tr
                  key={upload.id}
                  className={`border-b last:border-0 hover:bg-muted/30 ${upload.deleted_at ? "opacity-50" : ""}`}
                >
                  {/* Thumbnail / icon */}
                  <td className="p-3">
                    {upload.has_thumbnail ? (
                      <img
                        src={thumbUrl(upload.id)}
                        alt=""
                        className="h-8 w-8 rounded object-cover bg-muted"
                        loading="lazy"
                      />
                    ) : (
                      <div className="h-8 w-8 rounded bg-muted flex items-center justify-center">
                        {fileIcon(upload.content_type)}
                      </div>
                    )}
                  </td>
                  <td className="p-3">
                    <button
                      className="text-left hover:underline cursor-pointer font-medium truncate max-w-[200px] block"
                      onClick={() => setPreview(upload)}
                      title={upload.original_name}
                    >
                      {upload.original_name}
                    </button>
                    {upload.deleted_at && (
                      <span className="inline-flex items-center px-1.5 py-0.5 rounded text-xs bg-red-100 text-red-700 mt-0.5">
                        Deleted
                      </span>
                    )}
                  </td>
                  <td className="p-3 text-muted-foreground text-xs hidden md:table-cell">
                    {upload.content_type}
                  </td>
                  <td className="p-3 text-muted-foreground text-xs hidden md:table-cell">
                    {formatBytes(upload.size_bytes)}
                  </td>
                  <td className="p-3 text-muted-foreground text-xs hidden lg:table-cell">
                    {upload.entity_type && upload.entity_id
                      ? `${upload.entity_type}:${upload.entity_id}`
                      : "\u2014"}
                  </td>
                  <td className="p-3 text-muted-foreground text-xs hidden lg:table-cell">
                    {formatTime(upload.created_at)}
                  </td>
                  <td className="p-3 text-right">
                    {deleteId === upload.id ? (
                      <span className="inline-flex items-center gap-2">
                        <span className="text-xs text-muted-foreground">Delete?</span>
                        <Button
                          variant="destructive"
                          size="sm"
                          disabled={acting === upload.id}
                          onClick={() => handleDelete(upload.id)}
                        >
                          {acting === upload.id ? "Deleting..." : "Confirm"}
                        </Button>
                        <Button
                          variant="ghost"
                          size="sm"
                          onClick={() => setDeleteId(null)}
                        >
                          Cancel
                        </Button>
                      </span>
                    ) : (
                      <span className="inline-flex items-center gap-1">
                        <Button
                          variant="ghost"
                          size="sm"
                          title="Preview"
                          onClick={() => setPreview(upload)}
                        >
                          <Eye className="h-3.5 w-3.5" />
                        </Button>
                        <a href={fileUrl(upload.id)} target="_blank" rel="noopener noreferrer">
                          <Button variant="ghost" size="sm" title="Download">
                            <Download className="h-3.5 w-3.5" />
                          </Button>
                        </a>
                        {!upload.deleted_at && (
                          <Button
                            variant="ghost"
                            size="sm"
                            title="Delete"
                            onClick={() => setDeleteId(upload.id)}
                          >
                            <Trash2 className="h-3.5 w-3.5 text-red-500" />
                          </Button>
                        )}
                      </span>
                    )}
                  </td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>

      {/* Pagination */}
      <Pagination
        page={page}
        totalPages={totalPages}
        total={total}
        pageSize={pageSize}
        onPrev={() => setPage(page - 1)}
        onNext={() => setPage(page + 1)}
      />

      {/* Upload Dialog */}
      {showUpload && (
        <Dialog open={showUpload} onClose={closeUploadDialog}>
          <DialogHeader>
            <DialogTitle>Upload File</DialogTitle>
            <DialogCloseButton onClick={closeUploadDialog} />
          </DialogHeader>
          <DialogBody className="space-y-4">
            {uploadError && (
              <div className="rounded-md bg-red-50 border border-red-200 p-3 text-sm text-red-700">
                {uploadError}
              </div>
            )}

            <div
              className={`border-2 border-dashed rounded-lg p-8 text-center transition-colors ${
                dragOver
                  ? "border-primary bg-primary/5"
                  : "border-muted-foreground/25 hover:border-muted-foreground/50"
              }`}
              onDragOver={(e) => { e.preventDefault(); setDragOver(true); }}
              onDragLeave={() => setDragOver(false)}
              onDrop={handleDrop}
            >
              <Upload className="h-8 w-8 mx-auto mb-3 text-muted-foreground" />
              <p className="text-sm text-muted-foreground mb-2">
                Drag and drop files here, or
              </p>
              <label className="inline-flex">
                <input
                  type="file"
                  multiple
                  className="hidden"
                  onChange={handleFileSelect}
                />
                <span className="text-sm font-medium text-primary hover:underline cursor-pointer">
                  browse files
                </span>
              </label>
              <p className="text-xs text-muted-foreground mt-2">Max 50 MB per file</p>
            </div>

            {selectedFiles.length > 0 && (
              <div className="space-y-2">
                <p className="text-sm font-medium">
                  {selectedFiles.length} file{selectedFiles.length !== 1 ? "s" : ""} selected
                </p>
                <div className="max-h-40 overflow-y-auto space-y-1">
                  {selectedFiles.map((f, i) => (
                    <div key={i} className="flex items-center justify-between text-sm py-1 px-2 rounded bg-muted/50">
                      <span className="truncate mr-2">{f.name}</span>
                      <span className="text-xs text-muted-foreground whitespace-nowrap">{formatBytes(f.size)}</span>
                    </div>
                  ))}
                </div>
              </div>
            )}
          </DialogBody>
          <DialogFooter>
            <Button variant="outline" onClick={closeUploadDialog} disabled={uploading}>
              Cancel
            </Button>
            <Button onClick={handleUpload} disabled={uploading || selectedFiles.length === 0}>
              {uploading ? "Uploading..." : `Upload ${selectedFiles.length > 0 ? selectedFiles.length : ""} file${selectedFiles.length !== 1 ? "s" : ""}`}
            </Button>
          </DialogFooter>
        </Dialog>
      )}

      {/* Preview Dialog */}
      {preview && (
        <Dialog open={!!preview} onClose={() => setPreview(null)}>
          <DialogHeader>
            <DialogTitle className="truncate max-w-md">{preview.original_name}</DialogTitle>
            <DialogCloseButton onClick={() => setPreview(null)} />
          </DialogHeader>

          <DialogBody className="space-y-4">
            {/* Preview area */}
            {preview.content_type.startsWith("image/") ? (
              <div className="flex justify-center bg-muted rounded-lg p-4">
                <img
                  src={fileUrl(preview.id)}
                  alt={preview.original_name}
                  className="max-h-96 max-w-full object-contain rounded"
                />
              </div>
            ) : preview.content_type.startsWith("video/") ? (
              <div className="flex justify-center bg-muted rounded-lg p-4">
                <video
                  src={fileUrl(preview.id)}
                  controls
                  className="max-h-96 max-w-full rounded"
                />
              </div>
            ) : (
              <div className="flex flex-col items-center gap-3 py-8 bg-muted rounded-lg">
                {fileIcon(preview.content_type)}
                <p className="text-sm text-muted-foreground">No preview available</p>
              </div>
            )}

            {/* Metadata */}
            <div className="grid grid-cols-2 gap-x-6 gap-y-2 text-sm">
              <div>
                <span className="text-muted-foreground">Type</span>
                <p className="font-medium">{preview.content_type}</p>
              </div>
              <div>
                <span className="text-muted-foreground">Size</span>
                <p className="font-medium">{formatBytes(preview.size_bytes)}</p>
              </div>
              <div>
                <span className="text-muted-foreground">Owner</span>
                <p className="font-medium">
                  {preview.entity_type && preview.entity_id
                    ? `${preview.entity_type}:${preview.entity_id}`
                    : "\u2014"}
                </p>
              </div>
              <div>
                <span className="text-muted-foreground">Uploaded</span>
                <p className="font-medium">{formatTime(preview.created_at)}</p>
              </div>
              <div>
                <span className="text-muted-foreground">UUID</span>
                <p className="font-mono text-xs break-all">{preview.uuid}</p>
              </div>
              {preview.deleted_at && (
                <div>
                  <span className="text-muted-foreground">Deleted</span>
                  <p className="font-medium text-red-600">{formatTime(preview.deleted_at)}</p>
                </div>
              )}
            </div>
          </DialogBody>

          <DialogFooter>
            <a href={fileUrl(preview.id)} target="_blank" rel="noopener noreferrer">
              <Button variant="outline">
                <Download className="h-4 w-4 mr-2" />
                Download
              </Button>
            </a>
            <Button variant="outline" onClick={() => setPreview(null)}>
              Close
            </Button>
          </DialogFooter>
        </Dialog>
      )}
    </div>
  );
}
