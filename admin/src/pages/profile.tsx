import { useCallback, useEffect, useState } from "react";
import { toast } from "sonner";
import { get, put, del, ApiError } from "@/lib/api";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { ErrorAlert } from "@/components/ui/error-alert";
import { Spinner } from "@/components/ui/spinner";
import {
  Loader2,
  Monitor,
  Trash2,
} from "lucide-react";

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
      // Sessions are non-critical — don't block the page
    }
  }, []);

  useEffect(() => {
    Promise.all([loadProfile(), loadSessions()]).finally(() =>
      setLoading(false)
    );
  }, [loadProfile, loadSessions]);

  async function saveProfile() {
    setSavingProfile(true);
    try {
      const data = await put<{ admin: AdminProfile }>("/admin/profile", { name });
      setProfile(data.admin);
      setName(data.admin.name);
      toast.success("Profile updated");
    } catch (err) {
      if (err instanceof ApiError) {
        toast.error(err.message);
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
      toast.success("Password changed. Other sessions have been revoked.");
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
      toast.success("Session revoked");
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Failed to revoke session");
    }
  }

  if (loading) return <Spinner />;

  return (
    <div className="p-6">
      <div className="mb-6">
        <h1 className="text-2xl font-semibold tracking-tight">Profile</h1>
        <p className="text-sm text-muted-foreground">
          Manage your account settings and sessions
        </p>
      </div>

      {error && (
        <ErrorAlert
          message={error}
          onRetry={loadProfile}
          onDismiss={() => setError(null)}
          className="mb-6"
        />
      )}

      <div className="space-y-6 max-w-2xl">
        {/* Profile Info */}
        <Card>
          <CardHeader className="pb-3">
            <CardTitle className="text-base">Account Information</CardTitle>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="space-y-2">
              <Label htmlFor="email">Email</Label>
              <Input
                id="email"
                value={profile?.email ?? ""}
                disabled
                className="bg-muted/50"
              />
              <p className="text-xs text-muted-foreground">
                Email cannot be changed
              </p>
            </div>
            <div className="space-y-2">
              <Label htmlFor="name">Display Name</Label>
              <Input
                id="name"
                value={name}
                onChange={(e) => setName(e.target.value)}
                placeholder="Your name"
              />
            </div>
            <div className="grid grid-cols-2 gap-4">
              <div className="space-y-2">
                <Label>Role</Label>
                <p className="text-sm capitalize">{profile?.role}</p>
              </div>
              <div className="space-y-2">
                <Label>Member Since</Label>
                <p className="text-sm">
                  {profile?.created_at
                    ? new Date(profile.created_at).toLocaleDateString()
                    : "—"}
                </p>
              </div>
            </div>
            {profile?.scopes && profile.scopes.length > 0 && (
              <div className="space-y-2">
                <Label>Scopes</Label>
                <div className="flex flex-wrap gap-1.5">
                  {profile.scopes.map((scope) => (
                    <span
                      key={scope}
                      className="inline-flex items-center rounded-full bg-muted px-2.5 py-0.5 text-xs font-medium"
                    >
                      {scope}
                    </span>
                  ))}
                </div>
              </div>
            )}
            <div className="flex justify-end pt-2">
              <Button
                onClick={saveProfile}
                disabled={savingProfile || name === profile?.name}
                size="sm"
              >
                {savingProfile && (
                  <Loader2 className="mr-2 h-3.5 w-3.5 animate-spin" />
                )}
                Save Changes
              </Button>
            </div>
          </CardContent>
        </Card>

        {/* Change Password */}
        <Card>
          <CardHeader className="pb-3">
            <CardTitle className="text-base">Change Password</CardTitle>
          </CardHeader>
          <CardContent className="space-y-4">
            {passwordError && (
              <div className="rounded-md bg-destructive/10 px-3 py-2 text-sm text-destructive">
                {passwordError}
              </div>
            )}
            <div className="space-y-2">
              <Label htmlFor="current-password">Current Password</Label>
              <Input
                id="current-password"
                type="password"
                value={currentPassword}
                onChange={(e) => setCurrentPassword(e.target.value)}
                placeholder="Enter current password"
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="new-password">New Password</Label>
              <Input
                id="new-password"
                type="password"
                value={newPassword}
                onChange={(e) => setNewPassword(e.target.value)}
                placeholder="At least 8 characters"
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="confirm-password">Confirm New Password</Label>
              <Input
                id="confirm-password"
                type="password"
                value={confirmPassword}
                onChange={(e) => setConfirmPassword(e.target.value)}
                placeholder="Confirm new password"
              />
            </div>
            <p className="text-xs text-muted-foreground">
              Changing your password will revoke all other active sessions.
            </p>
            <div className="flex justify-end pt-2">
              <Button
                onClick={changePassword}
                disabled={
                  savingPassword || !currentPassword || !newPassword || !confirmPassword
                }
                size="sm"
              >
                {savingPassword && (
                  <Loader2 className="mr-2 h-3.5 w-3.5 animate-spin" />
                )}
                Change Password
              </Button>
            </div>
          </CardContent>
        </Card>

        {/* Active Sessions */}
        <Card>
          <CardHeader className="pb-3">
            <CardTitle className="text-base">
              Active Sessions
              <span className="ml-2 text-sm font-normal text-muted-foreground">
                ({sessions.length})
              </span>
            </CardTitle>
          </CardHeader>
          <CardContent className="p-0">
            {sessions.length === 0 ? (
              <p className="px-6 py-8 text-center text-sm text-muted-foreground">
                No active sessions
              </p>
            ) : (
              <div className="divide-y divide-border">
                {sessions.map((session) => (
                  <div
                    key={session.id}
                    className="flex items-center justify-between px-6 py-3"
                  >
                    <div className="flex items-start gap-3 min-w-0">
                      <div className="mt-0.5">
                        <Monitor className="h-4 w-4 text-muted-foreground" />
                      </div>
                      <div className="min-w-0">
                        <p className="text-sm">
                          Session
                          {session.current && (
                            <span className="ml-2 inline-flex items-center rounded-full bg-green-100 px-2 py-0.5 text-xs font-medium text-green-800 dark:bg-green-900/30 dark:text-green-400">
                              Current
                            </span>
                          )}
                        </p>
                        <p className="text-xs text-muted-foreground">
                          Created {formatRelative(session.created_at)} · Expires {formatRelative(session.expires_at)}
                        </p>
                      </div>
                    </div>
                    {!session.current && (
                      <Button
                        variant="ghost"
                        size="icon"
                        className="h-7 w-7 shrink-0 text-muted-foreground hover:text-destructive"
                        onClick={() => revokeSession(session.id)}
                      >
                        <Trash2 className="h-3.5 w-3.5" />
                      </Button>
                    )}
                  </div>
                ))}
              </div>
            )}
          </CardContent>
        </Card>
      </div>
    </div>
  );
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
