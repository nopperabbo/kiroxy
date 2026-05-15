import { test, expect } from "@playwright/test";

test.describe("topbar navigation", () => {
  for (const view of [
    "live",
    "pool",
    "metrics",
    "logs",
    "models",
    "tools",
    "settings",
  ]) {
    test(`hash routing to #view=${view} renders the matching view`, async ({ page }) => {
      await page.goto(`/dashboard-mansion#view=${view}`);

      const section = page.locator(`main section.view`);
      await expect(section).toHaveClass(new RegExp(`view--${view}`));
    });
  }

  test("clicking a tab updates URL hash and active state", async ({ page }) => {
    await page.goto("/dashboard-mansion");

    await page.locator(".nav__tab", { hasText: /metrics/i }).click();
    await expect(page).toHaveURL(/#view=metrics/);
    await expect(
      page.locator(".nav__tab.nav__tab--active", { hasText: /metrics/i }),
    ).toBeVisible();
  });
});
