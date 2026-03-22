import { lazy, Suspense } from "react";
import { BrowserRouter, Routes, Route, Navigate } from "react-router";
import { AuthProvider } from "@/lib/auth";
import { ErrorBoundary } from "@/components/error-boundary";
import { ProtectedRoute } from "@/components/protected-route";
import {
  CardPageSkeleton,
  DashboardSkeleton,
  DetailPageSkeleton,
  ListPageSkeleton,
} from "@/components/skeletons";
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

const D = <DashboardSkeleton />;
const Li = <ListPageSkeleton />;
const De = <DetailPageSkeleton />;
const C = <CardPageSkeleton />;

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
            <Route index element={<Suspense fallback={D}><DashboardPage /></Suspense>} />
            <Route path="users" element={<Suspense fallback={Li}><UsersPage /></Suspense>} />
            <Route path="users/:id" element={<Suspense fallback={De}><UserDetailPage /></Suspense>} />
            <Route path="admins" element={<Suspense fallback={Li}><AdminsPage /></Suspense>} />
            <Route path="admins/:id" element={<Suspense fallback={De}><AdminDetailPage /></Suspense>} />
            <Route path="sessions" element={<Suspense fallback={Li}><SessionsPage /></Suspense>} />
            <Route path="cron" element={<Suspense fallback={C}><CronPage /></Suspense>} />
            <Route path="queue" element={<Suspense fallback={C}><QueuePage /></Suspense>} />
            <Route path="logs" element={<Suspense fallback={C}><LogsPage /></Suspense>} />
            <Route path="database" element={<Suspense fallback={C}><DatabasePage /></Suspense>} />
            <Route path="api-keys" element={<Suspense fallback={Li}><ApiKeysPage /></Suspense>} />
            <Route path="audit" element={<Suspense fallback={Li}><AuditPage /></Suspense>} />
            <Route path="notifications" element={<Suspense fallback={Li}><NotificationsPage /></Suspense>} />
            <Route path="roles" element={<Suspense fallback={Li}><RolesPage /></Suspense>} />
            <Route path="uploads" element={<Suspense fallback={Li}><UploadsPage /></Suspense>} />
            <Route path="webhooks" element={<Suspense fallback={Li}><WebhooksPage /></Suspense>} />
            <Route path="webhooks/:id" element={<Suspense fallback={De}><WebhookDetailPage /></Suspense>} />
            <Route path="settings" element={<Suspense fallback={C}><SettingsPage /></Suspense>} />
            <Route path="profile" element={<Suspense fallback={C}><ProfilePage /></Suspense>} />
          </Route>
          <Route path="*" element={<Navigate to="/" replace />} />
        </Routes>
        </ErrorBoundary>
      </AuthProvider>
    </BrowserRouter>
  );
}
