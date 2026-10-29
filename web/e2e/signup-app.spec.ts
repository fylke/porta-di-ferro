import { test, expect, type Page } from '@playwright/test';
import { readFile, writeFile } from 'node:fs/promises';
import { pathToFileURL } from 'node:url';

// The participant's signup app (issues #91 and #5), in a real browser and opened the way
// a participant opens it: from a file, with no server behind it. The Go tests cover the
// formats and the import; only a browser can say that the page itself works from a
// downloads folder and writes the file the import expects.

// The definition as the event writes it (#142): its disciplines by name, the programme to
// read, and name, club and email asked of everybody.
const definition = {
  format: 'porta.signup',
  version: 2,
  definitionId: 'msl-open-2026',
  event: { name: 'MSL Open', venue: 'Linköping' },
  schedule: [
    { type: 'discipline', label: 'Longsword pools', startsAt: '09:30' },
    { type: 'break', label: 'Lunch', startsAt: '12:00', endsAt: '13:00' },
  ],
  tournaments: [
    { id: 'longsword', label: 'Open Steel Longsword', startsAt: '09:30', capacity: 28 },
    { id: 'sabre', label: 'Open Sabre', startsAt: '13:00' },
  ],
  fields: ['name', 'club', 'email'],
  staffRoles: [
    { id: 'head-ref', label: 'Head referee', offered: true },
    { id: 'assistant-ref', label: 'Assistant referee', offered: true },
    { id: 'score-keeper', label: 'Score keeper', offered: true },
    { id: 'physician', label: 'Physician', offered: false },
  ],
};

/**
 * The app with the event written into it, as the server bakes it, saved to disk and
 * opened from there. Baked here rather than fetched, so the test needs no particular
 * event on the shared test server; the Go tests check the server's baking.
 */
async function openFromDisk(page: Page, path: string) {
  const app = await readFile(new URL('../signup/signup.html', import.meta.url), 'utf8');
  const slot = '<script id="definition" type="application/json">null</script>';
  expect(app).toContain(slot);
  await writeFile(path, app.replace(slot, `<script id="definition" type="application/json">${JSON.stringify(definition)}</script>`));

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
  test('offers to staff what you are not fencing, and writes it into the file', async ({ page }, info) => {
    const errors = await openFromDisk(page, info.outputPath('signup.html'));
    await expect(page.getByText('This signup form needs JavaScript to run.')).toHaveCount(0);

    // Down for everything by default: both disciplines, and every role but physician.
    await expect(page.locator('#staff-longsword')).toBeChecked();
    await expect(page.locator('#staff-sabre')).toBeChecked();
    await expect(page.locator('#role-head-ref')).toBeChecked();
    await expect(page.locator('#role-score-keeper')).toBeChecked();
    await expect(page.locator('#role-physician')).not.toBeChecked();

    await page.locator('#f-name').fill('Ada Example');
    await page.locator('#f-club').fill('Example HEMA');
    // Each discipline under its own name, with when it starts.
    await expect(page.locator('label.pick', { hasText: 'Open Sabre' }).first()).toContainText('from 13:00');

    // Entering the longsword takes it off the staff list.
    await page.locator('#pick-longsword').check();
    await expect(page.locator('#staff-longsword')).toHaveCount(0);
    await expect(page.locator('#staff-sabre')).toBeChecked();

    // Not comfortable as a head referee.
    await page.locator('#role-head-ref').uncheck();

    // No email, no file: it is who she is to the organizer (#142).
    const button = page.getByRole('button', { name: 'Save my signup file' });
    await expect(button).toBeDisabled();
    await page.locator('#f-email').fill('not an address');
    await expect(button).toBeDisabled();
    await page.locator('#f-email').fill(' Ada@Example.org ');
    await expect(button).toBeEnabled();

    const response = await save(page);
    expect(response.format).toBe('porta.signup.response');
    expect(response.definitionId).toBe('msl-open-2026');
    expect(response.version).toBe(2);
    expect(response.participant.name).toBe('Ada Example');
    expect(response.participant.email).toBe('ada@example.org');
    expect(response.entries).toEqual(['longsword']);
    expect(response.staff).toEqual({ tournaments: ['sabre'], roles: ['assistant-ref', 'score-keeper'] });
    expect(errors).toEqual([]);
  });

  test('takes somebody who only comes to help, and nobody who unticks everything', async ({ page }, info) => {
    await openFromDisk(page, info.outputPath('signup.html'));
    await page.locator('#f-name').fill('Finn Example');
    await page.locator('#f-email').fill('finn@example.org');

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

test.describe('A signup file preview without JavaScript', () => {
  test.use({ javaScriptEnabled: false, viewport: { width: 390, height: 844 } });

  test('explains how to open the form instead of showing an empty page', async ({ page }) => {
    const app = await readFile(new URL('../signup/signup.html', import.meta.url), 'utf8');
    await page.route('**/signup-preview.html', (route) => route.fulfill({ contentType: 'text/html', body: app }));
    await page.goto('/signup-preview.html');
    await expect(page.getByRole('heading', { name: 'Sign up' })).toBeVisible();
    await expect(page.getByText('This signup form needs JavaScript to run.')).toBeVisible();
    await expect(page.getByText(/Ask the organizer for a web link/)).toBeVisible();
    expect(await page.locator('body').evaluate((body) => body.scrollWidth)).toBeLessThanOrEqual(390);
  });
});
