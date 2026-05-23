import { test, expect, Page } from "@playwright/test";

async function dismissScrim(page: Page) {
  const scrim = page.locator(".wiz-scrim");
  if ((await scrim.count()) > 0) {
    await scrim.click({ force: true, timeout: 2_000 }).catch(() => {});
    await scrim.waitFor({ state: "hidden", timeout: 3_000 }).catch(() => {});
  }
  await page.waitForTimeout(200);
}

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
      await dismissScrim(page);
      const section = page.locator(`main section.view`);
      await expect(section).toHaveClass(expected);

      if (view === "logs" || view === "models" || view === "tools" || view === "settings") {
        await expect(section).toHaveAttribute("aria-label", view);
      }
    });
  }

  test("clicking a tab updates URL hash and active state", async ({ page }) => {
    await page.goto("/dashboard-mansion");
    await dismissScrim(page);
    await page.locator(".nav__tab", { hasText: /metrics/i }).click();
    await expect(page).toHaveURL(/#view=metrics/);
    await expect(
      page.locator(".nav__tab.nav__tab--active", { hasText: /metrics/i }),
    ).toBeVisible();
  });
});
