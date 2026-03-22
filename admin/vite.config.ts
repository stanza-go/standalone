import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import path from "path";

export default defineConfig(({ command }) => ({
  base: command === "build" ? "/admin/" : "/",
  plugins: [react()],
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "./src"),
    },
  },
  build: {
    rollupOptions: {
      output: {
        manualChunks: {
          "vendor-react": ["react", "react-dom", "react-router"],
          "vendor-mantine": [
            "@mantine/core",
            "@mantine/hooks",
            "@mantine/form",
            "@mantine/notifications",
            "@mantine/dates",
            "@mantine/dropzone",
            "@mantine/spotlight",
          ],
          "vendor-charts": ["recharts", "@mantine/charts"],
          "vendor-icons": ["@tabler/icons-react"],
        },
      },
    },
  },
  server: {
    port: 23706,
    proxy: {
      "/api/": {
        target: "http://localhost:23710",
        ws: true,
      },
    },
  },
}));
