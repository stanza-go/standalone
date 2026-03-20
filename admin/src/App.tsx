import { BrowserRouter, Routes, Route, Navigate } from "react-router";
import { AuthProvider } from "@/lib/auth";
import { ProtectedRoute } from "@/components/protected-route";
import SidebarLayout from "@/components/layout/sidebar";
import LoginPage from "@/pages/login";
import DashboardPage from "@/pages/dashboard";
import CronPage from "@/pages/cron";
import QueuePage from "@/pages/queue";
import AdminsPage from "@/pages/admins";
import SessionsPage from "@/pages/sessions";
import LogsPage from "@/pages/logs";

export default function App() {
  return (
    <BrowserRouter>
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
            <Route path="admins" element={<AdminsPage />} />
            <Route path="sessions" element={<SessionsPage />} />
            <Route path="cron" element={<CronPage />} />
            <Route path="queue" element={<QueuePage />} />
            <Route path="logs" element={<LogsPage />} />
          </Route>
          <Route path="*" element={<Navigate to="/" replace />} />
        </Routes>
      </AuthProvider>
    </BrowserRouter>
  );
}
