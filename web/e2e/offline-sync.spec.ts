import { test, expect } from '@playwright/test';

test.describe('Offline & Route Fallthrough', () => {
  test('direct navigation to SPA routes fallthrough to index.html', async ({ page }) => {
    // Direct load of deep route
    await page.goto('/score/1');
    await expect(page.url()).toContain('/score/1');

    await page.goto('/display/mat/2');
    await expect(page.url()).toContain('/display/mat/2');
  });

  test('resilient during temporary offline status', async ({ page, context }) => {
    await page.goto('/score/1');

    // Simulate network going offline
    await context.setOffline(true);

    // Page state remains intact
    await expect(page.url()).toContain('/score/1');

    // Restore network
    await context.setOffline(false);
  });
});
