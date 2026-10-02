import { test, expect } from '@playwright/test';

// Entering competitors at the desk, as fast as a volunteer types (issue found while
// reviving the organizer flow spec, #114).
//
// The name field used to be cleared when the server answered an add. A volunteer who had
// already started typing the next name lost it, and the add after that sent nothing --
// four names typed in a row came out as two.
test('a name typed while the last one is saving is kept', async ({ page }) => {
  await page.goto('/admin');
  const name = page.getByPlaceholder('Name');
  const club = page.getByPlaceholder('Club');
  const add = page.getByRole('button', { name: 'Add', exact: true });

  const before = Number((await page.locator('.count').first().textContent())?.match(/\d+/)?.[0] ?? 0);
  const names = ['Typed Ahead One', 'Typed Ahead Two', 'Typed Ahead Three', 'Typed Ahead Four'];
  for (const n of names) {
    // No waiting for the answer: the next name goes in while the last is in flight.
    await name.fill(n);
    await club.fill('Desk Club');
    await add.click();
  }

  await expect(page.locator('.count').first()).toContainText(`${before + names.length} entered`);
  for (const n of names) await expect(page.getByText(n, { exact: true })).toBeVisible();
  await expect(name).toHaveValue('');
});
