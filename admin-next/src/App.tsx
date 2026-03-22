import { lazy, Suspense } from "react";
import { BrowserRouter, Routes, Route, Navigate } from "react-router";
import { Center, Loader } from "@mantine/core";
import { AuthProvider } from "@/lib/auth";
import { ProtectedRoute } from "@/components/protected-route";
import Shell from "@/components/layout/shell";
import LoginPage from "@/pages/login";

const DashboardPage = lazy(() => import("@/pages/dashboard"));
const UsersPage = lazy(() => import("@/pages/users"));
const AdminsPage = lazy(() => import("@/pages/admins"));
const SessionsPage = lazy(() => import("@/pages/sessions"));
const ApiKeysPage = lazy(() => import("@/pages/api-keys"));
const RolesPage = lazy(() => import("@/pages/roles"));
const AuditPage = lazy(() => import("@/pages/audit"));
const NotificationsPage = lazy(() => import("@/pages/notifications"));
const UploadsPage = lazy(() => import("@/pages/uploads"));
const WebhooksPage = lazy(() => import("@/pages/webhooks"));

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
            <Route
              index
              element={
                <Suspense fallback={<Center pt="xl"><Loader /></Center>}>
                  <DashboardPage />
                </Suspense>
              }
            />
            <Route path="users" element={<Suspense fallback={<Center pt="xl"><Loader /></Center>}><UsersPage /></Suspense>} />
            <Route path="users/:id" element={<Placeholder name="User Detail" />} />
            <Route path="admins" element={<Suspense fallback={<Center pt="xl"><Loader /></Center>}><AdminsPage /></Suspense>} />
            <Route path="admins/:id" element={<Placeholder name="Admin Detail" />} />
            <Route path="sessions" element={<Suspense fallback={<Center pt="xl"><Loader /></Center>}><SessionsPage /></Suspense>} />
            <Route path="cron" element={<Placeholder name="Cron Jobs" />} />
            <Route path="queue" element={<Placeholder name="Job Queue" />} />
            <Route path="logs" element={<Placeholder name="Logs" />} />
            <Route path="database" element={<Placeholder name="Database" />} />
            <Route path="api-keys" element={<Suspense fallback={<Center pt="xl"><Loader /></Center>}><ApiKeysPage /></Suspense>} />
            <Route path="audit" element={<Suspense fallback={<Center pt="xl"><Loader /></Center>}><AuditPage /></Suspense>} />
            <Route path="notifications" element={<Suspense fallback={<Center pt="xl"><Loader /></Center>}><NotificationsPage /></Suspense>} />
            <Route path="roles" element={<Suspense fallback={<Center pt="xl"><Loader /></Center>}><RolesPage /></Suspense>} />
            <Route path="uploads" element={<Suspense fallback={<Center pt="xl"><Loader /></Center>}><UploadsPage /></Suspense>} />
            <Route path="webhooks" element={<Suspense fallback={<Center pt="xl"><Loader /></Center>}><WebhooksPage /></Suspense>} />
            <Route path="webhooks/:id" element={<Placeholder name="Webhook Detail" />} />
            <Route path="settings" element={<Placeholder name="Settings" />} />
            <Route path="profile" element={<Placeholder name="Profile" />} />
          </Route>
          <Route path="*" element={<Navigate to="/" replace />} />
        </Routes>
      </AuthProvider>
    </BrowserRouter>
  );
}
