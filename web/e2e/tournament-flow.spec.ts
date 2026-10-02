import { test, expect } from '@playwright/test';

test.describe('Tournament & Organizer Flow', () => {
  test('adds competitors, sets up mats, draws pools, and accesses scorekeeper', async ({ page }) => {
    // 1. Visit the organizer's page. It moved to /admin when the landing page became the
    //    participants' (issue #98); this spec still opened / and has failed since.
    await page.goto('/admin');
    await expect(page.locator('h1')).toContainText('Porta di Ferro');

    // 2. Add competitors
    const nameInput = page.getByPlaceholder('Name');
    const clubInput = page.getByPlaceholder('Club');
    const addButton = page.getByRole('button', { name: 'Add', exact: true });

    const competitors = [
      { name: 'Alice Smith', club: 'Iron Gate Fencing' },
      { name: 'Bob Jones', club: 'Iron Gate Fencing' },
      { name: 'Charlie Brown', club: 'Northern Sword' },
      { name: 'Diana Prince', club: 'Northern Sword' },
    ];

    // One at a time, as at the desk: the name field is cleared when the server answers,
    // so typing the next before it has would be typing into a field about to be emptied.
    for (const [i, c] of competitors.entries()) {
      await nameInput.fill(c.name);
      await clubInput.fill(c.club);
      await addButton.click();
      await expect(page.locator('.count')).toContainText(`${i + 1} entered`);
    }

    // Verify competitor list count
    await expect(page.locator('.count')).toContainText('4 entered');

    // 3. Draw pools
    const drawButton = page.getByRole('button', { name: /Draw the pools/i });
    await drawButton.click();

    // Verify pools are rendered
    await expect(page.getByRole('heading', { name: /Pool 1/i })).toBeVisible();

    // 4. Navigate to Scorekeeper entry
    await page.goto('/score');
    await expect(page.getByRole('heading', { name: 'Which mat?' })).toBeVisible();

    // Select Mat 1
    await page.getByRole('button', { name: 'Mat 1' }).click();
    await expect(page.url()).toContain('/score/1');
  });
});
