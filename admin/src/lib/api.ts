const BASE = "/api";

export class ApiError extends Error {
  status: number;
  fields: Record<string, string>;
  constructor(status: number, message: string, fields?: Record<string, string>) {
    super(message);
    this.name = "ApiError";
    this.status = status;
    this.fields = fields ?? {};
  }
}

async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
  const opts: RequestInit = {
    method,
    headers: { "Content-Type": "application/json" },
    credentials: "include",
  };
  if (body !== undefined) {
    opts.body = JSON.stringify(body);
  }
  const res = await fetch(`${BASE}${path}`, opts);
  const data = await res.json();
  if (!res.ok) {
    // Redirect to login on 401 from non-auth endpoints (auth endpoints handle 401 themselves)
    if (res.status === 401 && !path.startsWith("/admin/auth")) {
      const base = import.meta.env.BASE_URL.replace(/\/+$/, "") || "";
      window.location.href = base + "/login";
    }
    throw new ApiError(res.status, data.error ?? "Unknown error", data.fields);
  }
  return data as T;
}

export function post<T>(path: string, body?: unknown): Promise<T> {
  return request<T>("POST", path, body);
}

export function get<T>(path: string): Promise<T> {
  return request<T>("GET", path);
}

export function put<T>(path: string, body?: unknown): Promise<T> {
  return request<T>("PUT", path, body);
}

export function del<T>(path: string): Promise<T> {
  return request<T>("DELETE", path);
}

export async function downloadCSV(path: string): Promise<void> {
  const res = await fetch(`${BASE}${path}`, {
    method: "GET",
    credentials: "include",
  });
  if (!res.ok) {
    const data = await res.json().catch(() => ({ error: "Export failed" }));
    throw new ApiError(res.status, data.error ?? "Export failed", data.fields);
  }
  const disposition = res.headers.get("Content-Disposition") ?? "";
  const match = disposition.match(/filename=(.+)/);
  const filename = match ? match[1] : "export.csv";
  const blob = await res.blob();
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = filename;
  a.click();
  URL.revokeObjectURL(url);
}

export async function upload<T>(path: string, file: File, fields?: Record<string, string>): Promise<T> {
  const form = new FormData();
  form.append("file", file);
  if (fields) {
    for (const [k, v] of Object.entries(fields)) {
      form.append(k, v);
    }
  }
  const res = await fetch(`${BASE}${path}`, {
    method: "POST",
    credentials: "include",
    body: form,
  });
  const data = await res.json();
  if (!res.ok) {
    throw new ApiError(res.status, data.error ?? "Upload failed", data.fields);
  }
  return data as T;
}
