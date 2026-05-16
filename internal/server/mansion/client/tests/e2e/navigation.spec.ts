import { test, expect } from "@playwright/test";

test.describe("topbar navigation", () => {
  const viewClassMap: Record<string, RegExp> = {
    live: /view--live/,
    pool: /view--pool/,
    metrics: /view--metrics/,
    logs: /view--utility/,
    models: /view--utility/,
    tools: /view--utility/,
    settings: /view--utility/,
  };

  for (const [view, expected] of Object.entries(viewClassMap)) {
    test(`hash routing to #view=${view} renders the matching view`, async ({ page }) => {
      await page.goto(`/dashboard-mansion#view=${view}`);
      const section = page.locator(`main section.view`);
      await expect(section).toHaveClass(expected);

      if (view === "logs" || view === "models" || view === "tools" || view === "settings") {
        await expect(section).toHaveAttribute("aria-label", view);
      }
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
