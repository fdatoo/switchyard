import { expect, test } from "@playwright/test";

/**
 * Snapshot test: render the Time-machine page with stubbed ReplayService responses.
 *
 * Uses page.route to intercept Connect-JSON requests to /switchyard.replay.v1.ReplayService/*
 * so the test runs without a live daemon.
 *
 * Run `task web:e2e -- --update-snapshots` once to generate reference images;
 * subsequent runs perform pixel-comparison regression detection.
 *
 * Reference screenshots: web/e2e/__screenshots__/time-machine/
 */

const FIXTURE_EVENT_ID = "evt_fixture_001";

const FIXTURE_CHAIN = {
  events: [
    { eventId: "evt_fixture_001", seq: "101", causationId: "", kind: "state.updated", entityId: "light.kitchen", occurredAt: "2026-01-01T10:00:01.000Z" },
    { eventId: "evt_fixture_002", seq: "102", causationId: "evt_fixture_001", kind: "command.issued", entityId: "light.kitchen", occurredAt: "2026-01-01T10:00:02.000Z" },
    { eventId: "evt_fixture_003", seq: "103", causationId: "evt_fixture_002", kind: "state.updated", entityId: "light.kitchen", occurredAt: "2026-01-01T10:00:03.000Z" },
    { eventId: "evt_fixture_004", seq: "104", causationId: "evt_fixture_003", kind: "config.applied", entityId: "light.kitchen", occurredAt: "2026-01-01T10:00:04.000Z" },
    { eventId: "evt_fixture_005", seq: "105", causationId: "evt_fixture_004", kind: "state.updated", entityId: "light.kitchen", occurredAt: "2026-01-01T10:00:05.000Z" },
  ],
};

const FIXTURE_LOAD_AT_SEQ = {
  seq: "101",
  entities: [
    { entityId: "light.kitchen", fields: { on: "true", brightness: "64" } },
    { entityId: "climate.living", fields: { temperature: "22", mode: "heat" } },
  ],
  diff: {
    entityDiffs: [
      {
        entityId: "light.kitchen",
        fieldDiffs: [{ field: "brightness", was: "18", now: "64" }],
      },
    ],
  },
  eventId: "evt_fixture_001",
  kind: "state.updated",
  entityId: "light.kitchen",
  source: "driver.hue",
  causationId: "",
  correlationId: "corr_abc",
  emitter: "driver.hue",
  spanId: "span_001",
  occurredAt: "2026-01-01T10:00:01.000Z",
  payloadJson: '{"stateChanged":{"attributes":{"light":{"on":true,"brightness":64}}}}',
  whyInteresting: "",
};

async function stubReplayService(page: import("@playwright/test").Page) {
  await page.route("**/switchyard.replay.v1.ReplayService/CausationChain", async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify(FIXTURE_CHAIN),
    });
  });

  await page.route("**/switchyard.replay.v1.ReplayService/LoadAtSeq", async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify(FIXTURE_LOAD_AT_SEQ),
    });
  });
}

test.describe("Time-machine page", () => {
  test.beforeEach(async ({ page }) => {
    // Set developer theme for deterministic rendering
    await page.addInitScript(() => {
      window.localStorage.setItem(
        "sy.theme.v2",
        JSON.stringify({ language: "developer", mode: "dark" }),
      );
    });
    await stubReplayService(page);
  });

  test("renders 5 scrubber dots and correct position label at step 1", async ({ page }) => {
    await page.goto(`/_authed/time-machine/${FIXTURE_EVENT_ID}`);

    // Wait for scrubber to appear
    const scrubber = page.getByTestId("scrubber");
    await expect(scrubber).toBeVisible({ timeout: 10_000 });

    // 5 dots should be visible
    const dots = page.locator("[data-testid^='dot-']");
    await expect(dots).toHaveCount(5);

    // Position label should show "step 1 of 5"
    const posLabel = page.getByTestId("pos-label");
    await expect(posLabel).toContainText("step 1 of 5");

    // Center pane should default to "Affected only" mode button being active
    const affectedBtn = page.getByTestId("mode-btn-affected");
    await expect(affectedBtn).toHaveAttribute("aria-pressed", "true");

    // Screenshot 1
    await expect(page).toHaveScreenshot("time-machine/step-01.png", { fullPage: true });
  });

  test("clicking next twice shows step 3 of 5", async ({ page }) => {
    await page.goto(`/_authed/time-machine/${FIXTURE_EVENT_ID}`);

    const scrubber = page.getByTestId("scrubber");
    await expect(scrubber).toBeVisible({ timeout: 10_000 });

    // Click › twice
    const nextBtn = page.getByTestId("next-btn");
    await nextBtn.click();
    await nextBtn.click();

    // Position label should show "step 3 of 5"
    const posLabel = page.getByTestId("pos-label");
    await expect(posLabel).toContainText("step 3 of 5");

    // Screenshot 2
    await expect(page).toHaveScreenshot("time-machine/step-03.png", { fullPage: true });
  });
});
