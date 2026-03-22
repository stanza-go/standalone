import { Navigate } from "react-router";
import {
  AppShell,
  Group,
  NavLink,
  Skeleton,
  Stack,
  Text,
} from "@mantine/core";
import { useAuth } from "@/lib/auth";
import type { ReactNode } from "react";

function AuthLoadingSkeleton() {
  return (
    <AppShell
      header={{ height: 60 }}
      navbar={{ width: 260, breakpoint: "md" }}
      padding="md"
    >
      <AppShell.Header>
        <Group h="100%" px="md" justify="space-between">
          <Text fw={700} size="lg">Stanza Admin</Text>
          <Group gap="xs">
            <Skeleton height={34} width={34} radius="sm" />
            <Skeleton height={34} width={34} radius="sm" />
            <Skeleton height={34} width={34} radius="sm" />
          </Group>
        </Group>
      </AppShell.Header>
      <AppShell.Navbar p="xs">
        <Stack gap={4}>
          {Array.from({ length: 12 }, (_, i) => (
            <NavLink key={i} label={<Skeleton height={14} width={80 + (i % 3) * 20} />} disabled />
          ))}
        </Stack>
      </AppShell.Navbar>
      <AppShell.Main>
        <Stack>
          <Skeleton height={28} width={120} />
          <Group gap="sm">
            {Array.from({ length: 4 }, (_, i) => (
              <Skeleton key={i} height={80} style={{ flex: 1 }} radius="md" />
            ))}
          </Group>
        </Stack>
      </AppShell.Main>
    </AppShell>
  );
}

export function ProtectedRoute({ children }: { children: ReactNode }) {
  const { admin, loading } = useAuth();

  if (loading) {
    return <AuthLoadingSkeleton />;
  }

  if (!admin) {
    return <Navigate to="/login" replace />;
  }

  return <>{children}</>;
}
