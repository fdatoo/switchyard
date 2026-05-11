import { expect, test } from "@playwright/test";

/**
 * Snapshot tests for the ambient display renderer and pair page.
 *
 * These tests mock the SolarService and DisplayService to avoid needing
 * a running daemon. Three scenarios:
 *
 *   1. Midday: Solar table puts current time at solar noon → midday gradient.
 *   2. Sunset: Solar table 45 minutes before sunset → sunset gradient.
 *   3. Pair page: success state and error state.
 *
 * Reference images are generated on first run; subsequent runs detect regressions.
 *
 * NOTE: Run `task web:e2e` once to generate reference images.
 */

const DISPLAY_ID = "test-display-id";
const DISPLAY_TOKEN = "sydisp_test_token_abc";

// ---------------------------------------------------------------------------
// Helper: mock fetch for display + solar + activity calls
// ---------------------------------------------------------------------------

/**
 * Inject route mocks into the page so fetch calls return canned responses
 * without hitting the daemon.
 */
async function mockServicesForDisplay(
  page: Parameters<typeof test>[1]["page"],
  opts: { solarNowHour: number },
) {
  // Inject a per-display token into localStorage
  await page.addInitScript(
    ({ id, token }) => {
      window.localStorage.setItem(`sy.display.${id}.token`, token);
    },
    { id: DISPLAY_ID, token: DISPLAY_TOKEN },
  );

  // Intercept Connect-RPC calls
  await page.route("**/switchyard.display.v1.DisplayService/Get", (route) => {
    void route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        display: {
          id: DISPLAY_ID,
          deviceName: "Test Display",
          pageSlug: "home",
          alertThreshold: "ALERT_THRESHOLD_NONE",
          tileOverrides: {},
        },
      }),
    });
  });

  // Mock SolarService with time-controlled solar table
  await page.route("**/switchyard.solar.v1.SolarService/GetTable", (route) => {
    const today = new Date();
    today.setHours(0, 0, 0, 0);

    // sunrise at 7, noon at 12, sunset at 20
    const sunrise = new Date(today);
    sunrise.setHours(7, 0, 0, 0);
    const noon = new Date(today);
    noon.setHours(12, 0, 0, 0);
    const sunset = new Date(today);
    sunset.setHours(20, 0, 0, 0);

    // Override "current time" by setting system time
    void page.evaluate((h) => {
      // Override Date.now to return the given hour today
      const base = new Date();
      base.setHours(h, 0, 0, 0);
      const fakeNow = base.getTime();
      const realDateNow = Date.now.bind(Date);
      // Only stub Date.now (not constructor) so existing code still works
      Date.now = () => fakeNow;
      void realDateNow; // keep reference
    }, opts.solarNowHour);

    void route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        today: {
          sunrise: sunrise.toISOString(),
          solarNoon: noon.toISOString(),
          sunset: sunset.toISOString(),
          date: today.toISOString().slice(0, 10),
        },
        tomorrow: {
          sunrise: new Date(sunrise.getTime() + 86400000).toISOString(),
          solarNoon: new Date(noon.getTime() + 86400000).toISOString(),
          sunset: new Date(sunset.getTime() + 86400000).toISOString(),
          date: new Date(today.getTime() + 86400000).toISOString().slice(0, 10),
        },
      }),
    });
  });

  // Mock ActivityService (streaming) — return empty response immediately
  await page.route("**/switchyard.activity.v1.ActivityService/Stories", (route) => {
    void route.fulfill({ status: 200, contentType: "application/json", body: "" });
  });
}

// ---------------------------------------------------------------------------
// Scenario 1: Midday
// ---------------------------------------------------------------------------

test("Ambient display — midday: no AlertPill, midday gradient applied", async ({ page }) => {
  await mockServicesForDisplay(page, { solarNowHour: 12 });
  await page.goto(`/display/${DISPLAY_ID}`);

  // Wait for ambient root to be present
  await expect(page.locator("[data-language='ambient']")).toBeVisible({ timeout: 10_000 });

  // AlertPill should NOT be present (threshold is NONE)
  await expect(page.locator("[data-testid='alert-pill']")).not.toBeVisible();

  // Full-page screenshot
  await expect(page).toHaveScreenshot("ambient-snapshot/midday.png", {
    fullPage: true,
    animations: "disabled",
  });
});

// ---------------------------------------------------------------------------
// Scenario 2: Sunset
// ---------------------------------------------------------------------------

test("Ambient display — sunset: sunset gradient applied", async ({ page }) => {
  // 45 minutes before sunset (20:00) → 19:15 → hour 19
  await mockServicesForDisplay(page, { solarNowHour: 19 });
  await page.goto(`/display/${DISPLAY_ID}`);

  await expect(page.locator("[data-language='ambient']")).toBeVisible({ timeout: 10_000 });

  await expect(page).toHaveScreenshot("ambient-snapshot/sunset.png", {
    fullPage: true,
    animations: "disabled",
  });
});

// ---------------------------------------------------------------------------
// Scenario 3a: Pair page — success state
// ---------------------------------------------------------------------------

test("Pair page — success state: navigates to /display/<id> after code entry", async ({ page }) => {
  // Mock the DisplayService.RedeemPairCode call
  await page.route("**/switchyard.display.v1.DisplayService/RedeemPairCode", (route) => {
    void route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ display_id: "new-display-id", token: "sydisp_newtoken" }),
    });
  });

  await page.goto("/pair");

  // Page should have the code input
  await expect(page.locator("[data-testid='pair-code-input']")).toBeVisible({ timeout: 5_000 });

  await page.locator("[data-testid='pair-code-input']").fill("123456");
  await page.locator("[data-testid='pair-submit']").click();

  // After success, should navigate to /display/new-display-id
  await expect(page).toHaveURL(/\/display\/new-display-id/, { timeout: 5_000 });

  await expect(page).toHaveScreenshot("ambient-snapshot/pair-success.png", {
    fullPage: true,
    animations: "disabled",
  });
});

// ---------------------------------------------------------------------------
// Scenario 3b: Pair page — error state
// ---------------------------------------------------------------------------

test("Pair page — error state: shows error message on invalid code", async ({ page }) => {
  await page.route("**/switchyard.display.v1.DisplayService/RedeemPairCode", (route) => {
    void route.fulfill({ status: 404, contentType: "application/json", body: "{}" });
  });

  await page.goto("/pair");

  await expect(page.locator("[data-testid='pair-code-input']")).toBeVisible({ timeout: 5_000 });
  await page.locator("[data-testid='pair-code-input']").fill("999999");
  await page.locator("[data-testid='pair-submit']").click();

  // Error message should appear
  await expect(page.locator("[data-testid='pair-error']")).toBeVisible({ timeout: 5_000 });
  await expect(page.locator("[data-testid='pair-error']")).toContainText("Code not found or expired");

  await expect(page).toHaveScreenshot("ambient-snapshot/pair-error.png", {
    fullPage: true,
    animations: "disabled",
  });
});
