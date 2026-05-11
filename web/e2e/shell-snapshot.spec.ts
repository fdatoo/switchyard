import { expect, test } from "@playwright/test";

/**
 * Snapshot test: render the Home placeholder in all four themes and take
 * a full-page screenshot for visual regression detection.
 *
 * The test forces the theme via localStorage before navigating, so it
 * does not depend on a running switchyardd daemon.
 *
 * NOTE: These snapshots use `toMatchSnapshot` (Playwright's built-in
 * pixel-comparison). Run `task web:e2e` once to generate the reference
 * images, then subsequent runs will fail if the shell visually regresses.
 */

const THEMES = [
  "friendly-light",
  "friendly-dark",
  "ambient",
  "developer",
] as const;

for (const theme of THEMES) {
  test(`shell renders correctly in ${theme} theme`, async ({ page }) => {
    // Inject the theme preference into localStorage before the page loads
    await page.addInitScript((t) => {
      const language = t === "ambient" ? "ambient" : t === "developer" ? "developer" : "friendly";
      const mode = t === "friendly-dark" ? "dark" : t === "friendly-light" ? "light" : "system";
      window.localStorage.setItem("sy.theme.v2", JSON.stringify({ language, mode }));
    }, theme);

    // Navigate to the home placeholder page
    await page.goto("/_authed/home");

    // Wait for the shell to render
    await expect(page.getByTestId("shell")).toBeVisible({ timeout: 10_000 });

    // Verify the correct data-theme is set on documentElement
    await expect(page.locator("html")).toHaveAttribute("data-theme", theme);

    // Take a full-page screenshot for visual regression
    await expect(page).toHaveScreenshot(`shell-snapshot/${theme}.png`, {
      fullPage: true,
      animations: "disabled",
    });
  });
}
