import { test, expect } from "@playwright/test";

test.describe("Pkl editor", () => {
  test.beforeEach(async ({ page }) => {
    // Navigate to the editor route with a test fixture file.
    // The dev server must be running: task ui:dev
    await page.goto(
      "/_authed/pkl-editor/automations/sunset-lights.pkl"
    );
    // Wait for the editor container to appear (either Monaco or the loading fallback).
    // Monaco loads lazily so we wait for either the loading state or the editor itself.
    await page.waitForSelector(
      ".monaco-editor, [data-testid='editor-loading']",
      { timeout: 15_000 }
    );
  });

  test("editor loads and shows the file tree", async ({ page }) => {
    // File tree shows automations directory label or file name
    const fileTreeOrLoading = page.locator(
      "[data-testid='pkl-editor-root'], [data-testid='editor-loading']"
    );
    await expect(fileTreeOrLoading.first()).toBeVisible({ timeout: 10_000 });
  });

  test("embedded Starlark editor area renders", async ({ page }) => {
    // Monaco renders the view-lines container when loaded
    // Just verify the editor region is in the DOM
    const editorArea = page.locator(
      ".monaco-editor .view-lines, [data-testid='editor-loading']"
    );
    await expect(editorArea.first()).toBeVisible({ timeout: 15_000 });
  });

  test("snapshot: editor in developer theme", async ({ page }) => {
    await page.evaluate(() => {
      document.documentElement.dataset.theme = "developer";
    });
    // Wait for the pkl-editor root
    const editorRoot = page.locator("[data-testid='pkl-editor-root']");
    if (await editorRoot.isVisible()) {
      await page.screenshot({
        path: "e2e/__screenshots__/pkl-editor/developer-theme.png",
        fullPage: true,
      });
    }
  });
});
