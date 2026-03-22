import { lazy } from "react";
import { BrowserRouter, Routes, Route, Navigate } from "react-router";
import { AuthProvider } from "@/lib/auth";
import { ThemeProvider } from "@/lib/theme";
import { ErrorBoundary } from "@/components/ui/error-boundary";
import { ProtectedRoute } from "@/components/protected-route";
import { Toaster } from "@/components/ui/sonner";
import SidebarLayout from "@/components/layout/sidebar";
import LoginPage from "@/pages/login";

const DashboardPage = lazy(() => import("@/pages/dashboard"));
const CronPage = lazy(() => import("@/pages/cron"));
const QueuePage = lazy(() => import("@/pages/queue"));
const AdminsPage = lazy(() => import("@/pages/admins"));
const SessionsPage = lazy(() => import("@/pages/sessions"));
const LogsPage = lazy(() => import("@/pages/logs"));
const DatabasePage = lazy(() => import("@/pages/database"));
const SettingsPage = lazy(() => import("@/pages/settings"));
const UsersPage = lazy(() => import("@/pages/users"));
const APIKeysPage = lazy(() => import("@/pages/api-keys"));
const AuditPage = lazy(() => import("@/pages/audit"));
const NotificationsPage = lazy(() => import("@/pages/notifications"));
const RolesPage = lazy(() => import("@/pages/roles"));
const UploadsPage = lazy(() => import("@/pages/uploads"));
const ProfilePage = lazy(() => import("@/pages/profile"));
const UserDetailPage = lazy(() => import("@/pages/user-detail"));
const AdminDetailPage = lazy(() => import("@/pages/admin-detail"));
const WebhooksPage = lazy(() => import("@/pages/webhooks"));
const WebhookDetailPage = lazy(() => import("@/pages/webhook-detail"));

export default function App() {
  const basename = import.meta.env.BASE_URL.replace(/\/+$/, "") || undefined;

  return (
    <ThemeProvider>
    <ErrorBoundary>
    <BrowserRouter basename={basename}>
      <Toaster position="top-right" duration={3000} closeButton richColors />
      <AuthProvider>
        <Routes>
          <Route path="/login" element={<LoginPage />} />
          <Route
            element={
              <ProtectedRoute>
                <SidebarLayout />
              </ProtectedRoute>
            }
          >
            <Route index element={<DashboardPage />} />
            <Route path="users" element={<UsersPage />} />
            <Route path="users/:id" element={<UserDetailPage />} />
            <Route path="admins" element={<AdminsPage />} />
            <Route path="admins/:id" element={<AdminDetailPage />} />
            <Route path="sessions" element={<SessionsPage />} />
            <Route path="cron" element={<CronPage />} />
            <Route path="queue" element={<QueuePage />} />
            <Route path="logs" element={<LogsPage />} />
            <Route path="database" element={<DatabasePage />} />
            <Route path="api-keys" element={<APIKeysPage />} />
            <Route path="audit" element={<AuditPage />} />
            <Route path="notifications" element={<NotificationsPage />} />
            <Route path="roles" element={<RolesPage />} />
            <Route path="uploads" element={<UploadsPage />} />
            <Route path="webhooks" element={<WebhooksPage />} />
            <Route path="webhooks/:id" element={<WebhookDetailPage />} />
            <Route path="settings" element={<SettingsPage />} />
            <Route path="profile" element={<ProfilePage />} />
          </Route>
          <Route path="*" element={<Navigate to="/" replace />} />
        </Routes>
      </AuthProvider>
    </BrowserRouter>
    </ErrorBoundary>
    </ThemeProvider>
  );
}
