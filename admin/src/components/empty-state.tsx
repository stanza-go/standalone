import type { ReactNode } from "react";
import { Stack, Text, ThemeIcon } from "@mantine/core";

interface EmptyStateProps {
  icon: ReactNode;
  title: string;
  description?: string;
  action?: ReactNode;
}

export function EmptyState({ icon, title, description, action }: EmptyStateProps) {
  return (
    <Stack align="center" gap="xs" py={48}>
      <ThemeIcon variant="light" size={48} radius="xl" color="gray">
        {icon}
      </ThemeIcon>
      <Text fw={500} size="sm" mt="xs">
        {title}
      </Text>
      {description && (
        <Text size="sm" c="dimmed" ta="center" maw={300}>
          {description}
        </Text>
      )}
      {action && <div style={{ marginTop: 4 }}>{action}</div>}
    </Stack>
  );
}
