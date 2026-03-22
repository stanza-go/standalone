import { useEffect, useState } from "react";
import { Transition, Paper, Group, Text } from "@mantine/core";
import { IconWifiOff } from "@tabler/icons-react";

export function ConnectionStatus() {
  const [offline, setOffline] = useState(!navigator.onLine);

  useEffect(() => {
    const goOffline = () => setOffline(true);
    const goOnline = () => setOffline(false);
    window.addEventListener("offline", goOffline);
    window.addEventListener("online", goOnline);
    return () => {
      window.removeEventListener("offline", goOffline);
      window.removeEventListener("online", goOnline);
    };
  }, []);

  return (
    <Transition mounted={offline} transition="slide-down" duration={200}>
      {(styles) => (
        <Paper
          style={{
            ...styles,
            position: "fixed",
            top: 0,
            left: 0,
            right: 0,
            zIndex: 1000,
            borderRadius: 0,
          }}
          bg="red.6"
          p="xs"
        >
          <Group justify="center" gap="xs">
            <IconWifiOff size={16} color="white" />
            <Text size="sm" c="white" fw={500}>
              You are offline. Changes may not be saved.
            </Text>
          </Group>
        </Paper>
      )}
    </Transition>
  );
}
