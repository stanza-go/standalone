import { lazy, Suspense } from "react";
import { BrowserRouter, Routes, Route, Navigate } from "react-router";
import { Center, Loader } from "@mantine/core";
import { AuthProvider } from "@/lib/auth";
import { ProtectedRoute } from "@/components/protected-route";
import Shell from "@/components/layout/shell";
import LoginPage from "@/pages/login";

const DashboardPage = lazy(() => import("@/pages/dashboard"));

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
            <Route path="users" element={<Placeholder name="Users" />} />
            <Route path="users/:id" element={<Placeholder name="User Detail" />} />
            <Route path="admins" element={<Placeholder name="Admin Users" />} />
            <Route path="admins/:id" element={<Placeholder name="Admin Detail" />} />
            <Route path="sessions" element={<Placeholder name="Sessions" />} />
            <Route path="cron" element={<Placeholder name="Cron Jobs" />} />
            <Route path="queue" element={<Placeholder name="Job Queue" />} />
            <Route path="logs" element={<Placeholder name="Logs" />} />
            <Route path="database" element={<Placeholder name="Database" />} />
            <Route path="api-keys" element={<Placeholder name="API Keys" />} />
            <Route path="audit" element={<Placeholder name="Audit Log" />} />
            <Route path="notifications" element={<Placeholder name="Notifications" />} />
            <Route path="roles" element={<Placeholder name="Roles" />} />
            <Route path="uploads" element={<Placeholder name="Uploads" />} />
            <Route path="webhooks" element={<Placeholder name="Webhooks" />} />
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
