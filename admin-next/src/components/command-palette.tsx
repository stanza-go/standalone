import { useMemo } from "react";
import { useNavigate } from "react-router";
import {
  Spotlight,
  type SpotlightActionGroupData,
} from "@mantine/spotlight";
import { useMantineColorScheme } from "@mantine/core";
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
  IconSun,
  IconMoon,
  IconDeviceDesktop,
  IconLogout,
  IconSearch,
} from "@tabler/icons-react";
import { useAuth } from "@/lib/auth";

export function CommandPalette() {
  const navigate = useNavigate();
  const { colorScheme, setColorScheme } = useMantineColorScheme();
  const { logout } = useAuth();

  const actions: SpotlightActionGroupData[] = useMemo(() => {
    const go = (path: string) => () => navigate(path);

    const pages: SpotlightActionGroupData[] = [
      {
        group: "General",
        actions: [
          {
            id: "dashboard",
            label: "Dashboard",
            description: "System overview and stats",
            leftSection: <IconDashboard size={20} stroke={1.5} />,
            onClick: go("/"),
          },
          {
            id: "profile",
            label: "Profile",
            description: "Your account and password",
            leftSection: <IconUser size={20} stroke={1.5} />,
            onClick: go("/profile"),
          },
        ],
      },
      {
        group: "Users & Access",
        actions: [
          {
            id: "users",
            label: "Users",
            description: "End users and customers",
            keywords: "end users customers",
            leftSection: <IconUsers size={20} stroke={1.5} />,
            onClick: go("/users"),
          },
          {
            id: "admins",
            label: "Admin Users",
            description: "Administrators and staff",
            keywords: "administrators staff",
            leftSection: <IconUserShield size={20} stroke={1.5} />,
            onClick: go("/admins"),
          },
          {
            id: "sessions",
            label: "Sessions",
            description: "Active login sessions",
            keywords: "token login active",
            leftSection: <IconShieldLock size={20} stroke={1.5} />,
            onClick: go("/sessions"),
          },
          {
            id: "api-keys",
            label: "API Keys",
            description: "Programmatic access tokens",
            keywords: "token bearer programmatic",
            leftSection: <IconKey size={20} stroke={1.5} />,
            onClick: go("/api-keys"),
          },
          {
            id: "roles",
            label: "Roles",
            description: "Roles, scopes, and permissions",
            keywords: "role scope permission access",
            leftSection: <IconShieldLock size={20} stroke={1.5} />,
            onClick: go("/roles"),
          },
        ],
      },
      {
        group: "System",
        actions: [
          {
            id: "cron",
            label: "Cron Jobs",
            description: "Scheduled periodic tasks",
            keywords: "scheduler schedule task periodic",
            leftSection: <IconClock size={20} stroke={1.5} />,
            onClick: go("/cron"),
          },
          {
            id: "queue",
            label: "Job Queue",
            description: "Background workers and jobs",
            keywords: "background worker task job",
            leftSection: <IconStack2 size={20} stroke={1.5} />,
            onClick: go("/queue"),
          },
          {
            id: "logs",
            label: "Logs",
            description: "Live log viewer and streaming",
            keywords: "log viewer stream tail",
            leftSection: <IconFileText size={20} stroke={1.5} />,
            onClick: go("/logs"),
          },
          {
            id: "database",
            label: "Database",
            description: "SQLite stats, backups, migrations",
            keywords: "sqlite backup migration",
            leftSection: <IconDatabase size={20} stroke={1.5} />,
            onClick: go("/database"),
          },
          {
            id: "webhooks",
            label: "Webhooks",
            description: "Outgoing webhook endpoints",
            keywords: "webhook callback endpoint event subscription",
            leftSection: <IconWebhook size={20} stroke={1.5} />,
            onClick: go("/webhooks"),
          },
        ],
      },
      {
        group: "Content",
        actions: [
          {
            id: "uploads",
            label: "Uploads",
            description: "Files and media management",
            keywords: "file media image",
            leftSection: <IconUpload size={20} stroke={1.5} />,
            onClick: go("/uploads"),
          },
          {
            id: "notifications",
            label: "Notifications",
            description: "In-app alerts and messages",
            keywords: "alert message notify",
            leftSection: <IconBell size={20} stroke={1.5} />,
            onClick: go("/notifications"),
          },
          {
            id: "audit",
            label: "Audit Log",
            description: "Admin activity history",
            keywords: "audit trail history activity",
            leftSection: <IconHistory size={20} stroke={1.5} />,
            onClick: go("/audit"),
          },
        ],
      },
      {
        group: "Config",
        actions: [
          {
            id: "settings",
            label: "Settings",
            description: "Application configuration",
            keywords: "config configuration preference",
            leftSection: <IconSettings size={20} stroke={1.5} />,
            onClick: go("/settings"),
          },
        ],
      },
    ];

    // Quick actions — hide the current theme option
    const themeActions = [];
    if (colorScheme !== "light") {
      themeActions.push({
        id: "theme-light",
        label: "Switch to Light Mode",
        leftSection: <IconSun size={20} stroke={1.5} />,
        onClick: () => setColorScheme("light"),
      });
    }
    if (colorScheme !== "dark") {
      themeActions.push({
        id: "theme-dark",
        label: "Switch to Dark Mode",
        leftSection: <IconMoon size={20} stroke={1.5} />,
        onClick: () => setColorScheme("dark"),
      });
    }
    if (colorScheme !== "auto") {
      themeActions.push({
        id: "theme-auto",
        label: "Switch to System Theme",
        leftSection: <IconDeviceDesktop size={20} stroke={1.5} />,
        onClick: () => setColorScheme("auto"),
      });
    }
    themeActions.push({
      id: "logout",
      label: "Log Out",
      description: "End your session",
      leftSection: <IconLogout size={20} stroke={1.5} />,
      onClick: () => {
        logout();
        navigate("/login", { replace: true });
      },
    });

    pages.push({
      group: "Quick Actions",
      actions: themeActions,
    });

    return pages;
  }, [navigate, colorScheme, setColorScheme, logout]);

  return (
    <Spotlight
      actions={actions}
      nothingFound="No results found"
      highlightQuery
      shortcut={["mod + K"]}
      searchProps={{
        leftSection: <IconSearch size={20} stroke={1.5} />,
        placeholder: "Search pages and actions...",
      }}
    />
  );
}
