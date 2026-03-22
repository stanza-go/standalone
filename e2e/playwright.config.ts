import { defineConfig } from "@playwright/test";

const BASE_URL =
  process.env.BASE_URL ||
  "https://standalone-production-cc35.up.railway.app";

const AUTH_FILE = "auth.json";

export default defineConfig({
  testDir: ".",
  testMatch: ["auth.setup.ts", "admin.spec.ts"],
  timeout: 30_000,
  expect: { timeout: 10_000 },
  fullyParallel: false,
  retries: 0,
  reporter: [["list"]],
  use: {
    baseURL: BASE_URL,
    screenshot: "only-on-failure",
    trace: "off",
  },
  projects: [
    {
      name: "auth",
      testMatch: "auth.setup.ts",
      timeout: 120_000, // may wait for rate limit window to expire
    },
    {
      name: "e2e",
      testMatch: "admin.spec.ts",
      dependencies: ["auth"],
      use: { storageState: AUTH_FILE },
    },
  ],
});
