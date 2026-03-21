import { Sun, Moon, Monitor } from "lucide-react";
import { useTheme } from "@/lib/theme";
import { Button } from "@/components/ui/button";

const cycle: Array<"light" | "dark" | "system"> = ["light", "dark", "system"];

export function ThemeToggle({ collapsed }: { collapsed?: boolean }) {
  const { theme, setTheme, resolved } = useTheme();

  const next = () => {
    const idx = cycle.indexOf(theme);
    setTheme(cycle[(idx + 1) % cycle.length]);
  };

  const icon =
    theme === "system" ? (
      <Monitor className="h-4 w-4" />
    ) : resolved === "dark" ? (
      <Moon className="h-4 w-4" />
    ) : (
      <Sun className="h-4 w-4" />
    );

  const label =
    theme === "light" ? "Light" : theme === "dark" ? "Dark" : "System";

  return (
    <Button
      variant="ghost"
      size={collapsed ? "icon" : "sm"}
      onClick={next}
      title={`Theme: ${label}`}
      className="text-muted-foreground"
    >
      {icon}
      {!collapsed && <span className="text-xs">{label}</span>}
    </Button>
  );
}
