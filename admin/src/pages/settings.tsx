import { useEffect, useState } from "react";
import { get, put } from "@/lib/api";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Check, Loader2, RefreshCw, X } from "lucide-react";

interface Setting {
  key: string;
  value: string;
  group_name: string;
  updated_at: string;
}

interface SettingsResponse {
  settings: Setting[];
}

export default function SettingsPage() {
  const [settings, setSettings] = useState<Setting[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [editingKey, setEditingKey] = useState<string | null>(null);
  const [editValue, setEditValue] = useState("");
  const [saving, setSaving] = useState(false);
  const [successKey, setSuccessKey] = useState<string | null>(null);

  async function load() {
    try {
      const data = await get<SettingsResponse>("/admin/settings");
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
    setSuccessKey(null);
  }

  function cancelEdit() {
    setEditingKey(null);
    setEditValue("");
  }

  async function saveEdit(key: string) {
    setSaving(true);
    try {
      const updated = await put<Setting>(`/admin/settings/${key}`, {
        value: editValue,
      });
      setSettings((prev) =>
        prev.map((s) => (s.key === key ? updated : s))
      );
      setEditingKey(null);
      setEditValue("");
      setSuccessKey(key);
      setTimeout(() => setSuccessKey(null), 2000);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to save");
    } finally {
      setSaving(false);
    }
  }

  // Group settings by group_name.
  const grouped = settings.reduce<Record<string, Setting[]>>((acc, s) => {
    if (!acc[s.group_name]) {
      acc[s.group_name] = [];
    }
    acc[s.group_name].push(s);
    return acc;
  }, {});

  const groupOrder = Object.keys(grouped).sort((a, b) => {
    if (a === "general") return -1;
    if (b === "general") return 1;
    return a.localeCompare(b);
  });

  return (
    <div className="p-6">
      <div className="mb-6 flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight">Settings</h1>
          <p className="text-sm text-muted-foreground">
            Application settings stored in the database
          </p>
        </div>
        <Button variant="outline" size="sm" onClick={load}>
          <RefreshCw className="mr-2 h-3.5 w-3.5" />
          Refresh
        </Button>
      </div>

      {error && (
        <div className="mb-6 rounded-md border border-destructive/50 bg-destructive/5 p-3 text-sm text-destructive">
          {error}
        </div>
      )}

      {settings.length === 0 && !error && (
        <div className="text-sm text-muted-foreground">Loading...</div>
      )}

      <div className="space-y-6">
        {groupOrder.map((group) => (
          <Card key={group}>
            <CardHeader className="pb-3">
              <CardTitle className="text-sm font-medium uppercase tracking-wider text-muted-foreground">
                {group}
              </CardTitle>
            </CardHeader>
            <CardContent className="p-0">
              <div className="divide-y divide-border">
                {grouped[group].map((s) => (
                  <div
                    key={s.key}
                    className="flex items-center justify-between px-6 py-3"
                  >
                    <div className="min-w-0 flex-1">
                      <p className="text-sm font-medium font-mono">{s.key}</p>
                      <p className="text-xs text-muted-foreground mt-0.5">
                        Updated {new Date(s.updated_at).toLocaleString()}
                      </p>
                    </div>
                    <div className="ml-4 flex items-center gap-2">
                      {editingKey === s.key ? (
                        <>
                          <input
                            type="text"
                            value={editValue}
                            onChange={(e) => setEditValue(e.target.value)}
                            onKeyDown={(e) => {
                              if (e.key === "Enter") saveEdit(s.key);
                              if (e.key === "Escape") cancelEdit();
                            }}
                            autoFocus
                            className="h-8 w-48 rounded-md border border-input bg-background px-2 text-sm focus:outline-none focus:ring-1 focus:ring-ring"
                          />
                          <Button
                            variant="ghost"
                            size="icon"
                            className="h-7 w-7"
                            onClick={() => saveEdit(s.key)}
                            disabled={saving}
                          >
                            {saving ? (
                              <Loader2 className="h-3.5 w-3.5 animate-spin" />
                            ) : (
                              <Check className="h-3.5 w-3.5" />
                            )}
                          </Button>
                          <Button
                            variant="ghost"
                            size="icon"
                            className="h-7 w-7"
                            onClick={cancelEdit}
                            disabled={saving}
                          >
                            <X className="h-3.5 w-3.5" />
                          </Button>
                        </>
                      ) : (
                        <>
                          <span
                            className={
                              "cursor-pointer rounded px-2 py-1 text-sm font-mono hover:bg-accent transition-colors" +
                              (successKey === s.key
                                ? " text-green-600"
                                : "")
                            }
                            onClick={() => startEdit(s)}
                          >
                            {s.value}
                          </span>
                        </>
                      )}
                    </div>
                  </div>
                ))}
              </div>
            </CardContent>
          </Card>
        ))}
      </div>
    </div>
  );
}
