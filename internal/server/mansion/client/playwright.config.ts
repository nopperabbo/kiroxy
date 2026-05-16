// Playwright config for Mansion E2E tests.
//
// Strategy: tests run against a running kiroxy binary at :8787 (the
// dashboard is embedded in the Go server, not served by Vite). Locally
// the user runs `./kiroxy serve` separately; CI starts it in a
// background step before invoking `pnpm test:e2e`.
//
// We don't auto-spawn the binary from this config because that requires
// duplicating the build pipeline (Vite → Go embed → binary). Keeping it
// external means tests are simple and the same config works for local
// + CI without conditional logic.

import { defineConfig, devices } from "@playwright/test";

const BASE_URL = process.env.MANSION_BASE_URL || "http://localhost:8787";

export default defineConfig({
  testDir: "./tests/e2e",
  outputDir: "./test-results",
  timeout: 30_000,
  expect: { timeout: 5_000 },
  fullyParallel: false,
  retries: process.env.CI ? 1 : 0,
  workers: 1,
  reporter: process.env.CI ? [["github"], ["list"]] : [["list"]],
  use: {
    baseURL: BASE_URL,
    trace: "retain-on-failure",
    screenshot: "only-on-failure",
  },
  projects: [
    {
      name: "desktop-chromium",
      use: {
        ...devices["Desktop Chrome"],
        viewport: { width: 1440, height: 900 },
      },
    },
    {
      name: "mobile-iphone",
      use: {
        ...devices["Pixel 7"],
        viewport: { width: 390, height: 844 },
        deviceScaleFactor: 3,
        isMobile: true,
        hasTouch: true,
      },
    },
  ],
});
