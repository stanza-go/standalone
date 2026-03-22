import { useCallback, useState } from "react";
import { Outlet, useLocation, useNavigate } from "react-router";
import {
  AppShell,
  Burger,
  Group,
  NavLink,
  ScrollArea,
  Text,
  UnstyledButton,
  ActionIcon,
  Tooltip,
  useMantineColorScheme,
} from "@mantine/core";
import { useDisclosure } from "@mantine/hooks";
import {
  IconDashboard,
  IconUsers,
  IconShieldLock,
  IconKey,
  IconUserShield,
  IconClock,
  IconStack2,
  IconFileText,
  IconDatabase,
  IconWebhook,
  IconUpload,
  IconBell,
  IconHistory,
  IconSettings,
  IconUser,
  IconLogout,
  IconSun,
  IconMoon,
  IconDeviceDesktop,
  IconSearch,
} from "@tabler/icons-react";
import { useAuth } from "@/lib/auth";
import { NotificationBell } from "@/components/notification-bell";
import { spotlight } from "@mantine/spotlight";
import { CommandPalette } from "@/components/command-palette";

interface NavItem {
  label: string;
  icon: React.FC<{ size?: number; stroke?: number }>;
  path: string;
}

interface NavSection {
  label?: string;
  items: NavItem[];
}

const sections: NavSection[] = [
  {
    items: [
      { label: "Dashboard", icon: IconDashboard, path: "/" },
    ],
  },
  {
    label: "Users & Access",
    items: [
      { label: "Users", icon: IconUsers, path: "/users" },
      { label: "Admin Users", icon: IconUserShield, path: "/admins" },
      { label: "Sessions", icon: IconShieldLock, path: "/sessions" },
      { label: "API Keys", icon: IconKey, path: "/api-keys" },
      { label: "Roles", icon: IconShieldLock, path: "/roles" },
    ],
  },
  {
    label: "System",
    items: [
      { label: "Cron Jobs", icon: IconClock, path: "/cron" },
      { label: "Job Queue", icon: IconStack2, path: "/queue" },
      { label: "Logs", icon: IconFileText, path: "/logs" },
      { label: "Database", icon: IconDatabase, path: "/database" },
      { label: "Webhooks", icon: IconWebhook, path: "/webhooks" },
    ],
  },
  {
    label: "Content",
    items: [
      { label: "Uploads", icon: IconUpload, path: "/uploads" },
      { label: "Notifications", icon: IconBell, path: "/notifications" },
      { label: "Audit Log", icon: IconHistory, path: "/audit" },
    ],
  },
  {
    label: "Config",
    items: [
      { label: "Settings", icon: IconSettings, path: "/settings" },
    ],
  },
];

function ThemeToggle() {
  const { colorScheme, setColorScheme } = useMantineColorScheme();

  const next = useCallback(() => {
    const order: Array<"light" | "dark" | "auto"> = ["light", "dark", "auto"];
    const idx = order.indexOf(colorScheme);
    setColorScheme(order[(idx + 1) % order.length]!);
  }, [colorScheme, setColorScheme]);

  const icon =
    colorScheme === "dark" ? (
      <IconMoon size={18} />
    ) : colorScheme === "light" ? (
      <IconSun size={18} />
    ) : (
      <IconDeviceDesktop size={18} />
    );

  const label =
    colorScheme === "dark" ? "Dark" : colorScheme === "light" ? "Light" : "System";

  return (
    <Tooltip label={`Theme: ${label}`}>
      <ActionIcon variant="default" onClick={next} size="lg">
        {icon}
      </ActionIcon>
    </Tooltip>
  );
}

export default function Shell() {
  const [opened, { toggle, close }] = useDisclosure();
  const { admin, logout } = useAuth();
  const navigate = useNavigate();
  const location = useLocation();
  const [loggingOut, setLoggingOut] = useState(false);

  const isActive = (path: string) => {
    if (path === "/") return location.pathname === "/";
    return location.pathname.startsWith(path);
  };

  const handleLogout = async () => {
    setLoggingOut(true);
    await logout();
    navigate("/login", { replace: true });
  };

  return (
    <AppShell
      header={{ height: 60 }}
      navbar={{
        width: 260,
        breakpoint: "md",
        collapsed: { mobile: !opened },
      }}
      padding="md"
    >
      <AppShell.Header>
        <Group h="100%" px="md" justify="space-between">
          <Group>
            <Burger opened={opened} onClick={toggle} hiddenFrom="md" size="sm" />
            <Text fw={700} size="lg">
              Stanza Admin
            </Text>
          </Group>
          <Group gap="xs">
            <Tooltip label="Search (⌘K)">
              <ActionIcon variant="default" size="lg" onClick={() => spotlight.open()}>
                <IconSearch size={18} stroke={1.5} />
              </ActionIcon>
            </Tooltip>
            <NotificationBell />
            <ThemeToggle />
          </Group>
        </Group>
      </AppShell.Header>

      <AppShell.Navbar>
        <AppShell.Section grow component={ScrollArea}>
          {sections.map((section, si) => (
            <div key={si}>
              {section.label && (
                <Text
                  size="xs"
                  fw={700}
                  c="dimmed"
                  tt="uppercase"
                  px="md"
                  pt={si === 0 ? "sm" : "lg"}
                  pb={4}
                >
                  {section.label}
                </Text>
              )}
              {section.items.map((item) => (
                <NavLink
                  key={item.path}
                  label={item.label}
                  leftSection={<item.icon size={18} stroke={1.5} />}
                  active={isActive(item.path)}
                  onClick={() => {
                    navigate(item.path);
                    close();
                  }}
                  variant="light"
                />
              ))}
            </div>
          ))}
        </AppShell.Section>

        <AppShell.Section>
          <div style={{ borderTop: "1px solid var(--mantine-color-default-border)", padding: "var(--mantine-spacing-sm) var(--mantine-spacing-md)" }}>
            <Group justify="space-between">
              <UnstyledButton onClick={() => { navigate("/profile"); close(); }}>
                <Group gap="xs">
                  <IconUser size={18} stroke={1.5} />
                  <div>
                    <Text size="sm" fw={500} lh={1.2}>
                      {admin?.name || "Admin"}
                    </Text>
                    <Text size="xs" c="dimmed" lh={1.2}>
                      {admin?.role || "admin"}
                    </Text>
                  </div>
                </Group>
              </UnstyledButton>
              <Tooltip label="Log out">
                <ActionIcon
                  variant="default"
                  onClick={handleLogout}
                  loading={loggingOut}
                  size="lg"
                >
                  <IconLogout size={18} stroke={1.5} />
                </ActionIcon>
              </Tooltip>
            </Group>
          </div>
        </AppShell.Section>
      </AppShell.Navbar>

      <AppShell.Main>
        <Outlet />
      </AppShell.Main>

      <CommandPalette />
    </AppShell>
  );
}
