import { expect, test } from "@playwright/test";

/**
 * Snapshot test: render the Home page in friendly-light and friendly-dark
 * and take a full-page screenshot for visual regression detection.
 *
 * The test forces the theme via localStorage before navigating, so it
 * does not depend on any specific daemon state beyond authentication.
 *
 * NOTE: Run `task web:e2e` once to generate the reference images, then
 * subsequent runs will fail if the Home page visually regresses.
 */

const HOME_THEMES = ["friendly-light", "friendly-dark"] as const;

for (const theme of HOME_THEMES) {
  test(`Home renders in ${theme} without visual regressions`, async ({ page }) => {
    // Inject theme preference into localStorage before the page loads
    await page.addInitScript((t) => {
      const mode = t === "friendly-dark" ? "dark" : "light";
      window.localStorage.setItem("sy.theme.v2", JSON.stringify({ language: "friendly", mode }));
    }, theme);

    // Navigate to the Home page
    await page.goto("/_authed/home");

    // Wait for the greeting heading to be visible
    await expect(page.getByRole("heading", { level: 1 })).toBeVisible({ timeout: 10_000 });

    // Verify the correct data-theme is set on documentElement
    await expect(page.locator("html")).toHaveAttribute("data-theme", theme);

    // Take a full-page screenshot for visual regression
    await expect(page).toHaveScreenshot(`home-snapshot/home-${theme}.png`, {
      fullPage: true,
      animations: "disabled",
    });
  });
}
