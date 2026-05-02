import { test, expect } from "@playwright/test";
import { SessionsPage } from "./pages/sessions-page";

test.describe("Navigation", () => {
  let sp: SessionsPage;

  test.beforeEach(async ({ page }) => {
    sp = new SessionsPage(page);
    await sp.goto();
  });

  test("keyboard ] navigates to next session", async () => {
    await sp.sessionItems.first().click();
    await expect(sp.sessionItems.first()).toHaveClass(/active/);

    await sp.pressNextSessionShortcut();
    await expect(sp.sessionItems.nth(1)).toHaveClass(/active/);
  });

  test("keyboard [ navigates to previous session", async () => {
    await sp.sessionItems.nth(1).click();
    await expect(sp.sessionItems.nth(1)).toHaveClass(/active/);

    await sp.pressPreviousSessionShortcut();
    await expect(sp.sessionItems.first()).toHaveClass(/active/);
  });

  test("analytics page shows when no session selected", async () => {
    await expect(sp.analyticsPage).toBeVisible();
    await expect(sp.analyticsToolbar).toBeVisible();
    await expect(sp.exportBtn).toContainText("Export CSV");
  });
});
