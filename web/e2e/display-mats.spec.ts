import { test, expect } from '@playwright/test';

test.describe('Display & Spectator Views', () => {
  test('renders display views cleanly', async ({ page }) => {
    // 1. Visit /display/mats
    await page.goto('/display/mats');
    await expect(page).toHaveURL('/display/mats');

    // 2. Visit /display/mat/1
    await page.goto('/display/mat/1');
    await expect(page).toHaveURL('/display/mat/1');

    // 3. Visit /display/roster
    await page.goto('/display/roster');
    await expect(page).toHaveURL('/display/roster');
    await expect(page.getByRole('heading', { name: 'Roster' })).toBeVisible();

    // 4. Visit /print/pools
    await page.goto('/print/pools');
    await expect(page).toHaveURL('/print/pools');
  });
});
