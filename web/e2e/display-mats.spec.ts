import { test, expect } from '@playwright/test';
import { aDiscipline } from './discipline';

test.describe('Display & Spectator Views', () => {
  test('renders display views cleanly', async ({ page, request }) => {
    // The roster and the pool sheets are a discipline's: with exactly one, the unprefixed
    // addresses are its own (#149).
    await aDiscipline(request);
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
    await expect(page.getByRole('heading', { name: 'How to calculate the eliminations' })).toBeVisible();
    await expect(page.getByText('Match point index (MPI)')).toBeVisible();

    await page.getByRole('button', { name: 'SV' }).click();
    await expect(page.getByRole('heading', { name: 'Så räknar du fram elimineringarna' })).toBeVisible();
    await expect(page.getByText('Matchpoängindex (MPI)')).toBeVisible();
  });
});
