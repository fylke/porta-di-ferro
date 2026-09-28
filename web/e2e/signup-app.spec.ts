import { test, expect, type Page } from '@playwright/test';
import { readFile, writeFile } from 'node:fs/promises';
import { pathToFileURL } from 'node:url';

// The participant's signup app (issues #91 and #5), in a real browser and opened the way
// a participant opens it: from a file, with no server behind it. The Go tests cover the
// formats and the import; only a browser can say that the page itself works from a
// downloads folder and writes the file the import expects.

const event = {
  signup: {
    definitionId: 'msl-open-2026',
    name: 'MSL Open',
    venue: 'Linköping',
    tournament: 'longsword',
  },
  schedule: [
    { at: '09:30', label: 'Open Steel Longsword', kind: 'discipline', tournament: 'longsword' },
    { at: '12:00', ends: '13:00', label: 'Lunch', kind: 'break' },
    { at: '13:00', label: 'Open Sabre', kind: 'discipline', tournament: 'sabre' },
  ],
};

async function openFromDisk(page: Page, request: import('@playwright/test').APIRequestContext, path: string) {
  expect((await request.put('/api/event', { data: event })).ok()).toBe(true);
  const res = await request.get('/api/signup/app.html?download=1');
  expect(res.ok()).toBe(true);
  await writeFile(path, await res.body());

  const errors: string[] = [];
  page.on('pageerror', (e) => errors.push(e.message));
  await page.goto(pathToFileURL(path).href);
  await expect(page.getByRole('heading', { name: 'MSL Open' })).toBeVisible();
  return errors;
}

async function save(page: Page) {
  const download = page.waitForEvent('download');
  await page.getByRole('button', { name: 'Save my signup file' }).click();
  return JSON.parse(await readFile(await (await download).path(), 'utf8'));
}

test.describe('The offline signup app', () => {
  test('offers to staff what you are not fencing, and writes it into the file', async ({
    page,
    request,
  }, info) => {
    const errors = await openFromDisk(page, request, info.outputPath('signup.html'));

    // Down for everything by default: both disciplines, and every role but physician.
    await expect(page.locator('#staff-longsword')).toBeChecked();
    await expect(page.locator('#staff-sabre')).toBeChecked();
    await expect(page.locator('#role-head-ref')).toBeChecked();
    await expect(page.locator('#role-score-keeper')).toBeChecked();
    await expect(page.locator('#role-physician')).not.toBeChecked();

    await page.locator('#f-name').fill('Ada Example');
    await page.locator('#f-club').fill('Example HEMA');

    // Entering the longsword takes it off the staff list.
    await page.locator('#pick-longsword').check();
    await expect(page.locator('#staff-longsword')).toHaveCount(0);
    await expect(page.locator('#staff-sabre')).toBeChecked();

    // Not comfortable as a head referee.
    await page.locator('#role-head-ref').uncheck();

    const response = await save(page);
    expect(response.format).toBe('porta.signup.response');
    expect(response.definitionId).toBe('msl-open-2026');
    expect(response.participant.name).toBe('Ada Example');
    expect(response.entries).toEqual(['longsword']);
    expect(response.staff).toEqual({ tournaments: ['sabre'], roles: ['assistant-ref', 'score-keeper'] });
    expect(errors).toEqual([]);
  });

  test('takes somebody who only comes to help, and nobody who unticks everything', async ({
    page,
    request,
  }, info) => {
    await openFromDisk(page, request, info.outputPath('signup.html'));
    await page.locator('#f-name').fill('Finn Example');

    const button = page.getByRole('button', { name: 'Save my signup file' });
    await expect(button).toBeEnabled();

    await page.locator('#staff-longsword').uncheck();
    await page.locator('#staff-sabre').uncheck();
    await expect(button).toBeDisabled();

    await page.locator('#staff-sabre').check();
    const response = await save(page);
    expect(response.entries).toEqual([]);
    expect(response.staff.tournaments).toEqual(['sabre']);
  });
});
