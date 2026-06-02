import { test, expect } from "@playwright/test";

test("overview page renders", async ({ page }) => {
  await page.goto("/");
  await expect(page.getByText("Crypto Research Console")).toBeVisible();
});
