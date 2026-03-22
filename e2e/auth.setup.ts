import { test as setup, expect } from "@playwright/test";

setup("authenticate as admin", async ({ page, baseURL }) => {
  // Retry login if rate-limited (20 req/min on auth endpoints).
  // This handles back-to-back test runs during development.
  for (let attempt = 0; attempt < 3; attempt++) {
    await page.goto(`${baseURL}/admin/login`);
    await page.getByLabel("Email").fill("admin@stanza.dev");
    await page.getByLabel("Password").fill("admin");
    await page.getByRole("button", { name: "Sign in" }).click();

    const dashboard = page.getByText("Dashboard");
    const rateLimited = page.getByText("too many requests");

    // Wait for either success or rate limit error
    await Promise.race([
      dashboard.waitFor({ state: "visible", timeout: 10_000 }),
      rateLimited.waitFor({ state: "visible", timeout: 10_000 }),
    ]).catch(() => {});

    if (await dashboard.isVisible().catch(() => false)) {
      await page.context().storageState({ path: "auth.json" });
      return;
    }

    // Rate-limited — wait for window to expire and retry
    // eslint-disable-next-line playwright/no-wait-for-timeout
    await page.waitForTimeout(30_000);
  }

  throw new Error("Failed to authenticate after 3 attempts");
});
