import { test, expect } from "@playwright/test";

test.describe("theme toggle", () => {
  test("light scheme attribute applies and changes body color", async ({ page }) => {
    await page.goto("/dashboard-mansion");

    const darkColor = await page.evaluate(() => {
      return getComputedStyle(document.body).color;
    });

    await page.evaluate(() => {
      document.documentElement.dataset.scheme = "light";
    });

    const lightColor = await page.evaluate(() => {
      return getComputedStyle(document.body).color;
    });

    expect(darkColor).not.toBe(lightColor);
  });

  test("dark scheme is the default on first load", async ({ page }) => {
    await page.goto("/dashboard-mansion");
    const scheme = await page.evaluate(() => document.documentElement.dataset.scheme);
    expect(scheme === "dark" || scheme === undefined || scheme === "").toBe(true);
  });
});
