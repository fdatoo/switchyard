import { test, expect } from "@playwright/test";

const MOBILE = { width: 390, height: 844 }; // iPhone 14

test.use({ viewport: MOBILE });

const SCREENS = [
  { name: "MobileHome", path: "/home" },
  { name: "Rooms", path: "/rooms" },
  { name: "Activity", path: "/activity" },
  { name: "AutomationView", path: "/automations/test-automation" },
  { name: "PklViewer", path: "/settings/pkl" },
];

for (const { name, path } of SCREENS) {
  test(`mobile snapshot — ${name}`, async ({ page }) => {
    await page.goto(path);
    await page.waitForLoadState("networkidle");
    await expect(page).toHaveScreenshot(`mobile-${name}.png`);
  });
}

test("mobile snapshot — SearchSheet open", async ({ page }) => {
  await page.goto("/home");
  await page.waitForLoadState("networkidle");
  await page.getByRole("button", { name: /search/i }).click();
  await page.waitForSelector('[role="searchbox"]');
  await expect(page).toHaveScreenshot("mobile-SearchSheet.png");
});

test("mobile snapshot — RoomSheet open", async ({ page }) => {
  await page.goto("/rooms");
  await page.waitForLoadState("networkidle");
  await page.getByText("Living Room").first().click();
  await expect(page.getByRole("slider")).toBeVisible();
  await expect(page).toHaveScreenshot("mobile-RoomSheet.png");
});
