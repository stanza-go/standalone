import { Component, type ErrorInfo, type ReactNode } from "react";
import {
  Button,
  Center,
  Code,
  Group,
  Paper,
  Stack,
  Text,
  Title,
} from "@mantine/core";
import { IconAlertTriangle, IconRefresh } from "@tabler/icons-react";

interface Props {
  children: ReactNode;
}

interface State {
  error: Error | null;
}

export class ErrorBoundary extends Component<Props, State> {
  state: State = { error: null };

  static getDerivedStateFromError(error: Error): State {
    return { error };
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    console.error("ErrorBoundary caught:", error, info.componentStack);
  }

  render() {
    if (!this.state.error) {
      return this.props.children;
    }

    const base = import.meta.env.BASE_URL.replace(/\/+$/, "") || "";

    return (
      <Center mih="100vh" p="xl">
        <Paper shadow="sm" p="xl" radius="md" withBorder maw={480} w="100%">
          <Stack align="center" gap="md">
            <IconAlertTriangle size={48} color="var(--mantine-color-red-6)" />
            <Title order={3}>Something went wrong</Title>
            <Text c="dimmed" ta="center" size="sm">
              An unexpected error occurred while rendering this page. You can try
              reloading or navigate back to the dashboard.
            </Text>
            <Code block w="100%" style={{ maxHeight: 120, overflow: "auto" }}>
              {this.state.error.message}
            </Code>
            <Group>
              <Button
                leftSection={<IconRefresh size={16} />}
                onClick={() => this.setState({ error: null })}
              >
                Try again
              </Button>
              <Button
                variant="default"
                component="a"
                href={base + "/"}
              >
                Dashboard
              </Button>
            </Group>
          </Stack>
        </Paper>
      </Center>
    );
  }
}
