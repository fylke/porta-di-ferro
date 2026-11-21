import { test, expect } from '@playwright/test';
import { aDiscipline } from './discipline';

test.describe('Display & Spectator Views', () => {
  // REQ: US_SPEC_01, US_SPEC_02, US_SPEC_03, FR_DISPLAY_01, FR_EXPORT_02, FR_I18N_01
  test('renders display views cleanly', async ({ page, request }) => {
    // The roster and the pool sheets are a discipline's, so they are asked for at its own
    // address: other specs add disciplines to the server they share, and the unprefixed
    // addresses answer only while there is exactly one (#149).
    const slug = await aDiscipline(request);
    // 1. Visit /display/mats
    await page.goto('/display/mats');
    await expect(page).toHaveURL('/display/mats');

    // 2. Visit /display/mat/1
    await page.goto('/display/mat/1');
    await expect(page).toHaveURL('/display/mat/1');

    // 3. Visit /display/roster
    await page.goto(`/d/${slug}/display/roster`);
    await expect(page).toHaveURL(`/d/${slug}/display/roster`);
    await expect(page.getByRole('heading', { name: 'Roster' })).toBeVisible();

    // 4. Visit /print/pools
    await page.goto(`/d/${slug}/print/pools`);
    await expect(page).toHaveURL(`/d/${slug}/print/pools`);
    await expect(page.getByRole('heading', { name: 'How to calculate the eliminations' })).toBeVisible();
    await expect(page.getByText('Match point index (MPI)')).toBeVisible();

    await page.getByRole('button', { name: 'SV' }).click();
    await expect(page.getByRole('heading', { name: 'Så räknar du fram elimineringarna' })).toBeVisible();
    await expect(page.getByText('Matchpoängindex (MPI)')).toBeVisible();
  });
});
