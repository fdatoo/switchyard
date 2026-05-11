import { expect, test } from "@playwright/test";

/**
 * Snapshot test: Settings → Drivers section with the Z2M card expanded.
 *
 * All network calls to DriverManagementService are intercepted with
 * page.route so the test never needs a running daemon.
 *
 * Run once with --update-snapshots to generate reference images, then
 * subsequent runs act as visual-regression guards.
 */

const Z2M_LOGS = [
  "2024-01-15T10:23:01.123Z info  Zigbee2MQTT started",
  "2024-01-15T10:23:01.456Z info  Coordinator firmware version: 20230507",
  "2024-01-15T10:23:02.001Z info  Loaded 42 devices from database",
  "2024-01-15T10:23:02.105Z info  MQTT connected to tcp://192.168.1.2:1883",
  "2024-01-15T10:23:03.400Z info  0x00158d0001ab12ef joined the network",
  "2024-01-15T10:23:04.800Z warn  Device 0x00158d0001ab12ef has low battery: 12%",
  "2024-01-15T10:23:08.100Z info  OTA update available for IKEA Tradfri bulb",
  "2024-01-15T10:23:10.200Z info  Permit join is OFF",
];

const LIST_RESPONSE = {
  running: [
    {
      id: "hue-bridge",
      pack: "@switchyard/hue",
      version: "1.4.2",
      status: "healthy",
      uptime_seconds: 86400,
      pid: 12340,
      socket: "/var/run/switchyard/hue-bridge.sock",
      config_file: "/etc/switchyard/drivers/hue.pkl",
      otel_span: "span-hue-001",
      entity_count: 24,
      events_per_day: 1200,
      last_cmd_ack_ms: 8,
      reconnects_today: 0,
      reconnecting_since: "",
    },
    {
      id: "z2m",
      pack: "@switchyard/z2m",
      version: "2.1.0",
      status: "reconnecting",
      uptime_seconds: 3721,
      pid: 12341,
      socket: "/var/run/switchyard/z2m.sock",
      config_file: "/etc/switchyard/drivers/z2m.pkl",
      otel_span: "span-z2m-002",
      entity_count: 42,
      events_per_day: 3800,
      last_cmd_ack_ms: 145,
      reconnects_today: 3,
      reconnecting_since: "2024-01-15T10:20:00Z",
    },
    {
      id: "esphome",
      pack: "@switchyard/esphome",
      version: "1.0.1",
      status: "healthy",
      uptime_seconds: 172800,
      pid: 12342,
      socket: "/var/run/switchyard/esphome.sock",
      config_file: "/etc/switchyard/drivers/esphome.pkl",
      otel_span: "span-esp-003",
      entity_count: 18,
      events_per_day: 960,
      last_cmd_ack_ms: 12,
      reconnects_today: 0,
      reconnecting_since: "",
    },
  ],
  available: [
    {
      id: "homekit-bridge",
      pack: "@switchyard/homekit",
      version: "1.1.0",
      status: "available",
    },
  ],
};

test("settings/drivers: Z2M card expanded — friendly-light snapshot", async ({ page }) => {
  // Inject friendly-light theme via localStorage before page loads
  await page.addInitScript(() => {
    window.localStorage.setItem(
      "sy.theme.v2",
      JSON.stringify({ language: "friendly", mode: "light" }),
    );
  });

  // Intercept DriverManagementService/List
  await page.route(
    "**/api/switchyard.driver.v1.DriverManagementService/List",
    async (route) => {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify(LIST_RESPONSE),
      });
    },
  );

  // Intercept DriverManagementService/Logs — only return Z2M log lines
  await page.route(
    "**/api/switchyard.driver.v1.DriverManagementService/Logs",
    async (route) => {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({ lines: Z2M_LOGS }),
      });
    },
  );

  // Navigate to the Drivers settings section
  await page.goto("/_authed/settings/drivers");

  // Wait for Z2M pack name to appear
  await expect(page.getByText("@switchyard/z2m")).toBeVisible({ timeout: 10_000 });

  // Find the Z2M card button (aria-expanded=false) and click it to expand
  const z2mButton = page.getByRole("button", { expanded: false }).filter({
    has: page.getByText("@switchyard/z2m"),
  });
  await z2mButton.click();

  // Wait for the log block to appear — indicates expanded detail is rendered
  await expect(
    page.getByText(Z2M_LOGS[0]!),
  ).toBeVisible({ timeout: 10_000 });

  // Take snapshot
  await expect(page).toHaveScreenshot(
    "settings-drivers-z2m-expanded-friendly-light.png",
    {
      fullPage: true,
      animations: "disabled",
    },
  );
});
