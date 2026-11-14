import { defineConfig } from "@playwright/test";

export default defineConfig({
  testDir: "./tests",
  workers: 1,
  use: {
    baseURL: "http://127.0.0.1:3101",
    trace: "retain-on-failure",
    launchOptions: { executablePath: process.env.PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH },
  },
  webServer: {
    command: "npm run build && cp -r .next/static .next/standalone/.next/ && node .next/standalone/server.js",
    env: { PORT: "3101", HOSTNAME: "127.0.0.1" },
    url: "http://127.0.0.1:3101/healthz",
    timeout: 120_000,
  },
});
