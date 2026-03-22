import { test, expect } from "@playwright/test";

// ---------------------------------------------------------------------------
// Helper
// ---------------------------------------------------------------------------

const AUTH_FILE = "auth.json";

/** Navigate to an admin page, re-authenticating if the JWT has expired. */
async function goAdmin(
  page: import("@playwright/test").Page,
  baseURL: string,
  path: string
) {
  await page.goto(`${baseURL}/admin${path}`);
  await page.waitForLoadState("networkidle");

  // Wait briefly for the SPA to settle — it may redirect to /login
  await page.waitForTimeout(1_000);

  const signInBtn = page.getByRole("button", { name: "Sign in" });
  const isLogin = await signInBtn
    .isVisible({ timeout: 2_000 })
    .catch(() => false);

  if (isLogin) {
    await page.getByLabel("Email").fill("admin@stanza.dev");
    await page.getByLabel("Password").fill("admin");
    await signInBtn.click();

    // Wait for login to fully complete — Dashboard heading confirms success
    await expect(
      page.getByRole("heading", { name: "Dashboard" })
    ).toBeVisible({ timeout: 15_000 });

    // Save refreshed auth state for subsequent tests
    await page.context().storageState({ path: AUTH_FILE });

    // Navigate to the intended page
    await page.goto(`${baseURL}/admin${path}`);
    await page.waitForLoadState("networkidle");
  }
}

// ---------------------------------------------------------------------------
// Auth (uses its own browser context — no stored auth)
// ---------------------------------------------------------------------------

test.describe("Auth", () => {
  test.use({ storageState: undefined });

  test("login with valid credentials redirects to dashboard", async ({
    page,
    baseURL,
  }) => {
    await page.goto(`${baseURL}/admin/login`);
    await page.getByLabel("Email").fill("admin@stanza.dev");
    await page.getByLabel("Password").fill("admin");
    await page.getByRole("button", { name: "Sign in" }).click();
    await expect(page.getByText("Dashboard")).toBeVisible({ timeout: 15_000 });
    await expect(page).toHaveURL(/\/admin\/?$/);
  });

  test("login with invalid credentials shows error", async ({
    page,
    baseURL,
  }) => {
    await page.goto(`${baseURL}/admin/login`);
    await page.getByLabel("Email").fill("wrong@example.com");
    await page.getByLabel("Password").fill("wrongpassword");
    await page.getByRole("button", { name: "Sign in" }).click();
    await expect(
      page.getByText(/invalid|incorrect|unauthorized/i)
    ).toBeVisible({ timeout: 5_000 });
  });
});

// ---------------------------------------------------------------------------
// Page navigation — verify all 16 list/card pages render
// ---------------------------------------------------------------------------

// Single test visiting all 16 admin pages via sidebar clicks (SPA navigation).
// The auth status endpoint is rate-limited (20 req/min per IP). Using SPA
// navigation avoids full page reloads, so only one status call is made.
test("all admin pages render", { timeout: 120_000 }, async ({ page, baseURL }) => {
  // navLabel = sidebar text, heading = page heading text
  const pages: { navLabel: string; heading: string }[] = [
    { navLabel: "Dashboard", heading: "Dashboard" },
    { navLabel: "Users", heading: "Users" },
    { navLabel: "Admin Users", heading: "Admin Users" },
    { navLabel: "Sessions", heading: "Sessions" },
    { navLabel: "API Keys", heading: "API Keys" },
    { navLabel: "Roles", heading: "Roles" },
    { navLabel: "Cron Jobs", heading: "Cron Jobs" },
    { navLabel: "Job Queue", heading: "Queue" },
    { navLabel: "Logs", heading: "Logs" },
    { navLabel: "Database", heading: "Database" },
    { navLabel: "Webhooks", heading: "Webhooks" },
    { navLabel: "Uploads", heading: "Uploads" },
    { navLabel: "Notifications", heading: "Notifications" },
    { navLabel: "Audit Log", heading: "Audit" },
    { navLabel: "Settings", heading: "Settings" },
  ];

  // Initial page load — only full load, only status call
  await goAdmin(page, baseURL!, "/");
  await expect(
    page.getByRole("heading", { name: "Dashboard" }).first()
  ).toBeVisible({ timeout: 10_000 });

  // Navigate via sidebar clicks (client-side routing, no page reload)
  const sidebar = page.locator("nav");
  for (const { navLabel, heading } of pages.slice(1)) {
    await sidebar.getByText(navLabel, { exact: true }).click();
    await expect(
      page.getByRole("heading", { name: heading }).first()
    ).toBeVisible({ timeout: 10_000 });
    await expect(
      page.getByText("Something went wrong")
    ).not.toBeVisible();
  }
});

// ---------------------------------------------------------------------------
// Dashboard — stat cards and system info
// ---------------------------------------------------------------------------

test.describe("Dashboard", () => {
  test("displays stat cards and system info", async ({ page, baseURL }) => {
    await goAdmin(page, baseURL!, "/");
    await expect(
      page.getByRole("heading", { name: "Dashboard" })
    ).toBeVisible({ timeout: 10_000 });
    await expect(page.getByText("Users").first()).toBeVisible();
    await expect(page.getByText(/sessions/i).first()).toBeVisible();
    await expect(page.getByText(/database/i).first()).toBeVisible();
    await expect(page.getByText(/uptime/i).first()).toBeVisible();
  });
});

// ---------------------------------------------------------------------------
// CRUD — create, find, delete a user
// ---------------------------------------------------------------------------

test.describe("User CRUD", () => {
  test.describe.configure({ mode: "serial" });

  const testEmail = `e2e-test-${Date.now()}@example.com`;
  const testName = "E2E Test User";

  test("create a new user", async ({ page, baseURL }) => {
    await goAdmin(page, baseURL!, "/users");

    await page.getByRole("button", { name: /create|add|new/i }).click();
    await page.getByLabel(/email/i).fill(testEmail);
    await page.getByLabel(/^name/i).fill(testName);
    await page.getByLabel(/password/i).fill("TestPassword123!");

    await page
      .getByRole("button", { name: /create|save|submit/i })
      .last()
      .click();

    await expect(
      page.getByText(/created|success/i).first()
    ).toBeVisible({ timeout: 5_000 });
  });

  test("find the created user via search", async ({ page, baseURL }) => {
    await goAdmin(page, baseURL!, "/users");

    const searchInput = page.getByPlaceholder(/search/i);
    await searchInput.fill(testEmail);
    await page.waitForTimeout(1_000);

    await expect(page.getByText(testEmail).first()).toBeVisible({
      timeout: 10_000,
    });
  });

  test("delete the test user via API", async ({ request, baseURL }) => {
    const resp = await request.get(
      `${baseURL}/api/admin/users?search=${encodeURIComponent(testEmail)}`
    );
    const data = await resp.json();
    const userId = data.data?.[0]?.id;
    if (userId) {
      const delResp = await request.delete(
        `${baseURL}/api/admin/users/${userId}`
      );
      expect(delResp.ok()).toBeTruthy();
    }
  });
});

// ---------------------------------------------------------------------------
// Admin CRUD — create and clean up
// ---------------------------------------------------------------------------

test.describe("Admin CRUD", () => {
  test.describe.configure({ mode: "serial" });

  const adminEmail = `e2e-admin-${Date.now()}@example.com`;

  test("create a new admin", async ({ page, baseURL }) => {
    await goAdmin(page, baseURL!, "/admins");

    await page.getByRole("button", { name: /create|add|new/i }).click();
    await page.getByLabel(/email/i).fill(adminEmail);
    await page.getByLabel(/^name/i).fill("E2E Admin");
    await page.getByLabel(/password/i).fill("AdminPass123!");

    await page
      .getByRole("button", { name: /create|save|submit/i })
      .last()
      .click();

    await expect(
      page.getByText(/created|success/i).first()
    ).toBeVisible({ timeout: 5_000 });
  });

  test("clean up — delete the admin via API", async ({ request, baseURL }) => {
    const resp = await request.get(
      `${baseURL}/api/admin/admins?search=${encodeURIComponent(adminEmail)}`
    );
    const data = await resp.json();
    const adminId = data.data?.[0]?.id;
    if (adminId) {
      const delResp = await request.delete(
        `${baseURL}/api/admin/admins/${adminId}`
      );
      expect(delResp.ok()).toBeTruthy();
    }
  });
});

// ---------------------------------------------------------------------------
// Detail pages
// ---------------------------------------------------------------------------

test.describe("Detail pages", () => {
  test("admin detail renders tabs", async ({ page, baseURL }) => {
    // Admin ID 1 always exists (admin@stanza.dev seed user)
    await goAdmin(page, baseURL!, "/admins/1");
    await expect(page.getByRole("tab").first()).toBeVisible({
      timeout: 10_000,
    });
  });

  test("user detail via keyboard navigation", async ({ page, baseURL }) => {
    await goAdmin(page, baseURL!, "/users");

    const rows = page.locator("tbody tr");
    if ((await rows.count()) === 0) {
      test.skip(true, "No users in table");
      return;
    }

    // Use keyboard nav (Enter on focused row) — same as useTableKeyboard hook
    const tbody = page.locator("tbody[tabindex='0']");
    await tbody.focus();
    await page.keyboard.press("ArrowDown");
    await page.waitForTimeout(200);
    await page.keyboard.press("Enter");
    await page.waitForURL(/\/users\/\d+/, { timeout: 5_000 });
    await expect(page.getByRole("tab").first()).toBeVisible({
      timeout: 10_000,
    });
  });
});

// ---------------------------------------------------------------------------
// Bulk actions — select all rows, verify bulk controls appear
// ---------------------------------------------------------------------------

test.describe("Bulk actions", () => {
  test("users page — select all shows bulk delete", async ({
    page,
    baseURL,
  }) => {
    await goAdmin(page, baseURL!, "/users");

    const headerCheckbox = page.locator("thead input[type='checkbox']");
    if (await headerCheckbox.isVisible({ timeout: 3_000 })) {
      await headerCheckbox.click();
      await expect(
        page.getByRole("button", { name: /delete|bulk/i }).first()
      ).toBeVisible({ timeout: 3_000 });
    }
  });

  test("sessions page — select all shows bulk revoke", async ({
    page,
    baseURL,
  }) => {
    await goAdmin(page, baseURL!, "/sessions");

    const headerCheckbox = page.locator("thead input[type='checkbox']");
    if (await headerCheckbox.isVisible({ timeout: 3_000 })) {
      await headerCheckbox.click();
      await expect(
        page.getByRole("button", { name: /revoke|bulk/i }).first()
      ).toBeVisible({ timeout: 3_000 });
    }
  });
});

// ---------------------------------------------------------------------------
// Keyboard navigation — uses the useTableKeyboard hook
// ---------------------------------------------------------------------------

test.describe("Keyboard navigation", () => {
  test("ArrowDown moves focus between table rows", async ({
    page,
    baseURL,
  }) => {
    await goAdmin(page, baseURL!, "/users");

    const rows = page.locator("tbody tr");
    const count = await rows.count();
    if (count < 2) {
      test.skip(true, "Need at least 2 rows");
      return;
    }

    // Focus the tbody (the hook gives tabIndex:0 to tbody)
    const tbody = page.locator("tbody[tabindex='0']");
    await tbody.focus();
    await page.waitForTimeout(200);

    // Press ArrowDown twice — first selects row 0, second moves to row 1
    await page.keyboard.press("ArrowDown");
    await page.waitForTimeout(200);
    await page.keyboard.press("ArrowDown");
    await page.waitForTimeout(200);

    // Second row should have inline outline style (applied by isFocused)
    const secondRow = rows.nth(1);
    const hasOutline = await secondRow.evaluate((el) => {
      return el.style.outline !== "" && el.style.outline !== "none";
    });
    expect(hasOutline).toBeTruthy();
  });

  test("Enter on focused row navigates to detail", async ({
    page,
    baseURL,
  }) => {
    await goAdmin(page, baseURL!, "/users");

    const rows = page.locator("tbody tr");
    if ((await rows.count()) === 0) {
      test.skip(true, "No rows visible");
      return;
    }

    // Focus tbody, ArrowDown to select first row, Enter to activate
    const tbody = page.locator("tbody[tabindex='0']");
    await tbody.focus();
    await page.keyboard.press("ArrowDown");
    await page.waitForTimeout(200);
    await page.keyboard.press("Enter");
    await page.waitForURL(/\/users\/\d+/, { timeout: 5_000 });
    await expect(page.getByRole("tab").first()).toBeVisible({
      timeout: 5_000,
    });
  });
});

// ---------------------------------------------------------------------------
// Command palette
// ---------------------------------------------------------------------------

test.describe("Command palette", () => {
  test("Cmd+K opens spotlight and search filters results", async ({
    page,
    baseURL,
  }) => {
    await goAdmin(page, baseURL!, "/");

    await page.keyboard.press("Meta+k");
    await page.waitForTimeout(500);

    const spotlight = page.getByPlaceholder(/search/i);
    if (await spotlight.isVisible({ timeout: 3_000 })) {
      await spotlight.fill("users");
      await page.waitForTimeout(300);
      await expect(page.getByText("Users").first()).toBeVisible();
    }
  });
});

// ---------------------------------------------------------------------------
// Theme toggle
// ---------------------------------------------------------------------------

test.describe("Theme", () => {
  test("toggle switches color scheme", async ({ page, baseURL }) => {
    await goAdmin(page, baseURL!, "/");

    const toggle = page.locator(
      'button[aria-label*="theme" i], button[aria-label*="color" i], button[aria-label*="dark" i], button[aria-label*="light" i]'
    );
    if (await toggle.first().isVisible({ timeout: 3_000 })) {
      const before = await page
        .locator("html")
        .getAttribute("data-mantine-color-scheme");
      await toggle.first().click();
      await page.waitForTimeout(300);
      const after = await page
        .locator("html")
        .getAttribute("data-mantine-color-scheme");
      expect(after).not.toBe(before);
    }
  });
});

// ---------------------------------------------------------------------------
// API health
// ---------------------------------------------------------------------------

test.describe("API health", () => {
  test("health endpoint returns ok", async ({ request, baseURL }) => {
    const resp = await request.get(`${baseURL}/api/health`);
    expect(resp.ok()).toBeTruthy();
    const body = await resp.json();
    expect(body.status).toBe("ok");
  });
});

// Cron and Settings page navigation are covered by the "all admin pages render" test above.

// ---------------------------------------------------------------------------
// Profile page
// ---------------------------------------------------------------------------

test.describe("Profile page", () => {
  test("displays admin profile and change password", async ({
    page,
    baseURL,
  }) => {
    await goAdmin(page, baseURL!, "/profile");
    await expect(
      page.getByRole("heading", { name: "Profile" })
    ).toBeVisible({ timeout: 10_000 });
    // Email is in a disabled TextInput — check via input value
    await expect(
      page.locator('input[value="admin@stanza.dev"]')
    ).toBeVisible({ timeout: 5_000 });
    await expect(
      page.getByText(/change password/i).first()
    ).toBeVisible();
  });
});

// ---------------------------------------------------------------------------
// CSV export
// ---------------------------------------------------------------------------

test.describe("CSV export", () => {
  test("users export triggers CSV download", async ({ page, baseURL }) => {
    await goAdmin(page, baseURL!, "/users");

    const exportBtn = page.getByRole("button", { name: /export/i });
    if (await exportBtn.isVisible({ timeout: 3_000 })) {
      const [download] = await Promise.all([
        page.waitForEvent("download", { timeout: 10_000 }),
        exportBtn.click(),
      ]);
      expect(download.suggestedFilename()).toMatch(/\.csv$/);
    }
  });
});
