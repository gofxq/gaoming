import { defineConfig, devices } from "@playwright/test";

export default defineConfig({
  testDir: "./e2e",
  forbidOnly: Boolean(process.env.CI),
  retries: process.env.CI ? 1 : 0,
  use: {
    baseURL: "http://127.0.0.1:4173",
    trace: "retain-on-failure",
  },
  projects: [{ name: "chromium", use: { ...devices["Desktop Chrome"] } }],
  webServer: {
    command: "yarn dev --host 0.0.0.0 --port 4173 --strictPort",
    url: "http://127.0.0.1:4173",
    // Never drive a user's running dev server with test sessions.
    reuseExistingServer: false,
  },
});
