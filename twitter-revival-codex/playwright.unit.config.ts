import { defineConfig } from "@playwright/test";

export default defineConfig({
  testDir: "./apps/web/tests",
  testMatch: "**/api.spec.ts",
  outputDir: ".local/test-results-unit",
  workers: 1,
  reporter: "list",
});
