// Smoke test — proves the dashboard loads, renders the topbar with all
// 7 view tabs exposed, and connects to the Go server. This is the
// minimum that should pass before any other E2E test runs.
//
// If this fails, the server isn't running or the embed pipeline is
// broken; fix that before debugging downstream tests.

import { test, expect } from "@playwright/test";

test.describe("smoke", () => {
  test("dashboard loads and shows 7-tab topbar", async ({ page }) => {
    await page.goto("/dashboard-mansion");
    await expect(page).toHaveTitle(/kiroxy/i);

    // All 7 tabs must be visible. Order matters for muscle memory.
    const tabs = ["Live Stream", "Pool", "Metrics", "Logs", "Models", "Tools", "Settings"];
    for (const label of tabs) {
      await expect(page.locator(".nav__tab", { hasText: new RegExp(label, "i") }))
        .toBeVisible();
    }
  });

  test("healthz endpoint reachable from same origin", async ({ request }) => {
    const res = await request.get("/healthz");
    expect(res.ok()).toBe(true);
    const body = await res.json();
    expect(body.status).toBe("ok");
  });
});
