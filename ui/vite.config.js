import { defineConfig } from "vite";

export default defineConfig({
  server: {
    port: 23700,
    proxy: {
      "/api": {
        target: "http://localhost:23710",
        changeOrigin: true,
      },
    },
  },
});
