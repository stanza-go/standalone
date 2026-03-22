import { lazy, Suspense } from "react";
import { BrowserRouter, Routes, Route, Navigate } from "react-router";
import { Center, Loader } from "@mantine/core";
import { AuthProvider } from "@/lib/auth";
import { ErrorBoundary } from "@/components/error-boundary";
import { ProtectedRoute } from "@/components/protected-route";
import Shell from "@/components/layout/shell";
import LoginPage from "@/pages/login";

const DashboardPage = lazy(() => import("@/pages/dashboard"));
const UsersPage = lazy(() => import("@/pages/users"));
const UserDetailPage = lazy(() => import("@/pages/user-detail"));
const AdminsPage = lazy(() => import("@/pages/admins"));
const AdminDetailPage = lazy(() => import("@/pages/admin-detail"));
const SessionsPage = lazy(() => import("@/pages/sessions"));
const ApiKeysPage = lazy(() => import("@/pages/api-keys"));
const RolesPage = lazy(() => import("@/pages/roles"));
const AuditPage = lazy(() => import("@/pages/audit"));
const NotificationsPage = lazy(() => import("@/pages/notifications"));
const UploadsPage = lazy(() => import("@/pages/uploads"));
const WebhooksPage = lazy(() => import("@/pages/webhooks"));
const WebhookDetailPage = lazy(() => import("@/pages/webhook-detail"));
const DatabasePage = lazy(() => import("@/pages/database"));
const SettingsPage = lazy(() => import("@/pages/settings"));
const CronPage = lazy(() => import("@/pages/cron"));
const QueuePage = lazy(() => import("@/pages/queue"));
const LogsPage = lazy(() => import("@/pages/logs"));
const ProfilePage = lazy(() => import("@/pages/profile"));

const L = <Center pt="xl"><Loader /></Center>;

export default function App() {
  const basename = import.meta.env.BASE_URL.replace(/\/+$/, "") || undefined;

  return (
    <BrowserRouter basename={basename}>
      <AuthProvider>
        <ErrorBoundary>
        <Routes>
          <Route path="/login" element={<LoginPage />} />
          <Route
            element={
              <ProtectedRoute>
                <Shell />
              </ProtectedRoute>
            }
          >
            <Route index element={<Suspense fallback={L}><DashboardPage /></Suspense>} />
            <Route path="users" element={<Suspense fallback={L}><UsersPage /></Suspense>} />
            <Route path="users/:id" element={<Suspense fallback={L}><UserDetailPage /></Suspense>} />
            <Route path="admins" element={<Suspense fallback={L}><AdminsPage /></Suspense>} />
            <Route path="admins/:id" element={<Suspense fallback={L}><AdminDetailPage /></Suspense>} />
            <Route path="sessions" element={<Suspense fallback={L}><SessionsPage /></Suspense>} />
            <Route path="cron" element={<Suspense fallback={L}><CronPage /></Suspense>} />
            <Route path="queue" element={<Suspense fallback={L}><QueuePage /></Suspense>} />
            <Route path="logs" element={<Suspense fallback={L}><LogsPage /></Suspense>} />
            <Route path="database" element={<Suspense fallback={L}><DatabasePage /></Suspense>} />
            <Route path="api-keys" element={<Suspense fallback={L}><ApiKeysPage /></Suspense>} />
            <Route path="audit" element={<Suspense fallback={L}><AuditPage /></Suspense>} />
            <Route path="notifications" element={<Suspense fallback={L}><NotificationsPage /></Suspense>} />
            <Route path="roles" element={<Suspense fallback={L}><RolesPage /></Suspense>} />
            <Route path="uploads" element={<Suspense fallback={L}><UploadsPage /></Suspense>} />
            <Route path="webhooks" element={<Suspense fallback={L}><WebhooksPage /></Suspense>} />
            <Route path="webhooks/:id" element={<Suspense fallback={L}><WebhookDetailPage /></Suspense>} />
            <Route path="settings" element={<Suspense fallback={L}><SettingsPage /></Suspense>} />
            <Route path="profile" element={<Suspense fallback={L}><ProfilePage /></Suspense>} />
          </Route>
          <Route path="*" element={<Navigate to="/" replace />} />
        </Routes>
        </ErrorBoundary>
      </AuthProvider>
    </BrowserRouter>
  );
}
