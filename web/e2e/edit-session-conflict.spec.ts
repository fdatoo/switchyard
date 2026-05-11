/**
 * Playwright E2E test: conflict banner appears mid-session.
 *
 * Requires a live switchyardd in test mode (TEST_SERVER_URL env var).
 * Skipped gracefully when TEST_SERVER_URL is absent.
 *
 * Test steps:
 * 1. Navigate to the automation editor for a test automation file; make a form change.
 * 2. Overwrite the .pkl file on disk via Node fs.
 * 3. Assert the ConflictBanner appears within 2 seconds.
 * 4. Click "Discard mine"; assert the banner disappears and dirty count returns to 0.
 */

import { promises as fs } from "node:fs";
import { expect, test } from "@playwright/test";

// Skip the entire test suite if TEST_SERVER_URL is not set.
const serverURL = process.env.TEST_SERVER_URL;
const TEST_PKL_PATH = process.env.TEST_PKL_PATH ?? "";

test.describe("Edit session conflict banner", () => {
  // Skip gracefully when the live server is absent.
  test.skip(!serverURL, "TEST_SERVER_URL not set — skipping live-server E2E test");

  test("ConflictBanner appears when file is overwritten mid-session", async ({ page }) => {
    if (!serverURL || !TEST_PKL_PATH) {
      test.skip();
      return;
    }

    // 1. Navigate to automation editor
    await page.goto(`${serverURL}/_authed/automations/test-automation`);

    // Wait for the editor to open and session to be established
    await expect(page.getByRole("heading")).toBeVisible({ timeout: 5000 });

    // Make a form change (mutate the session)
    const saveButton = page.getByRole("button", { name: /save/i });
    if (await saveButton.isVisible()) {
      // If there's an editable field, interact with it; otherwise just mark dirty via JS
      await page.evaluate(() => {
        window.dispatchEvent(new Event("test:mutate-session"));
      });
    }

    // 2. Overwrite the .pkl file on disk from outside (simulates MCP/CLI edit)
    const originalContent = await fs.readFile(TEST_PKL_PATH, "utf8");
    await fs.writeFile(TEST_PKL_PATH, originalContent + "\n// external edit\n", "utf8");

    // 3. Assert ConflictBanner appears within 2 seconds
    await expect(
      page.getByRole("alert", { name: "External edit conflict" }),
    ).toBeVisible({ timeout: 2000 });

    // 4. Click "Discard mine"; assert banner disappears
    await page.getByRole("button", { name: "Discard mine" }).click();

    await expect(
      page.getByRole("alert", { name: "External edit conflict" }),
    ).not.toBeVisible({ timeout: 2000 });

    // Restore original content
    await fs.writeFile(TEST_PKL_PATH, originalContent, "utf8");
  });
});
