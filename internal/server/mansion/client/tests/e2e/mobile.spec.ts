import { test, expect } from "@playwright/test";

test.describe("mobile responsive", () => {
  test.skip(({ browserName }, testInfo) => {
    return testInfo.project.name !== "mobile-iphone";
  });

  for (const view of ["live", "pool", "metrics", "logs", "models", "tools", "settings"]) {
    test(`${view} view has no horizontal overflow at 390px`, async ({ page }) => {
      await page.goto(`/dashboard-mansion#view=${view}`);

      const overflow = await page.evaluate(() => {
        return {
          viewport: window.innerWidth,
          bodyScrollW: document.body.scrollWidth,
        };
      });

      expect(overflow.bodyScrollW).toBeLessThanOrEqual(overflow.viewport + 2);
    });
  }

  test("topbar collapses tab labels at <560px", async ({ page }) => {
    await page.goto("/dashboard-mansion");

    const visibleLabels = await page.evaluate(() => {
      const spans = document.querySelectorAll(".nav__tab > span:nth-of-type(2)");
      return [...spans].filter((s) => {
        return getComputedStyle(s as Element).display !== "none";
      }).length;
    });

    expect(visibleLabels).toBeLessThanOrEqual(1);
  });
});
