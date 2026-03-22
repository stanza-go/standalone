import { lazy, Suspense } from "react";
import { BrowserRouter, Routes, Route, Navigate } from "react-router";
import { Center, Loader } from "@mantine/core";
import { AuthProvider } from "@/lib/auth";
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
const ProfilePage = lazy(() => import("@/pages/profile"));

// Placeholder for pages not yet ported — shows page name
function Placeholder({ name }: { name: string }) {
  return (
    <Center pt="xl">
      <div style={{ textAlign: "center" }}>
        <h2>{name}</h2>
        <p style={{ color: "var(--mantine-color-dimmed)" }}>
          This page will be ported from the shadcn/ui admin panel.
        </p>
      </div>
    </Center>
  );
}

const L = <Center pt="xl"><Loader /></Center>;

export default function App() {
  const basename = import.meta.env.BASE_URL.replace(/\/+$/, "") || undefined;

  return (
    <BrowserRouter basename={basename}>
      <AuthProvider>
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
            <Route path="cron" element={<Placeholder name="Cron Jobs" />} />
            <Route path="queue" element={<Placeholder name="Job Queue" />} />
            <Route path="logs" element={<Placeholder name="Logs" />} />
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
      </AuthProvider>
    </BrowserRouter>
  );
}
