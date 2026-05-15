import { test, expect } from "@playwright/test";

test.describe("metrics view essentials", () => {
  test.beforeEach(async ({ page }) => {
    await page.goto("/dashboard-mansion#view=metrics");
    await page.waitForSelector(".series-chart, .metrics-empty", { timeout: 5_000 });
  });

  test("renders 3 time-series cards or graceful empty state", async ({ page }) => {
    const charts = page.locator(".series-chart");
    const empty = page.locator(".metrics-empty");

    const chartCount = await charts.count();
    const emptyCount = await empty.count();

    expect(chartCount === 3 || emptyCount > 0).toBe(true);
  });

  test("range segmented control exposes 3 options", async ({ page }) => {
    const buttons = page.locator(".range__seg");
    await expect(buttons).toHaveCount(3);

    const labels = await buttons.allTextContents();
    expect(labels.map((l) => l.trim().toLowerCase())).toEqual(["5m", "1h", "24h"]);
  });

  test("clicking a range button updates active state", async ({ page }) => {
    const oneHourBtn = page.locator(".range__seg", { hasText: /^1h$/i });
    await oneHourBtn.click();
    await expect(oneHourBtn).toHaveClass(/range__seg--active/);
  });
});
