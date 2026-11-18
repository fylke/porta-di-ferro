import { test, expect } from '@playwright/test';
import { aDiscipline } from './discipline';

/**
 * #161: a browser allows six connections to one address, and a page used to hold a stream
 * for the event, the mats and each discipline it shows, so a few organizer tabs used them
 * all and every tab stopped answering. Each page now holds one. Five of the organizer's
 * pages side by side -- seven streams the old way -- and they can still ask the server
 * something.
 */
test('the organizer’s pages side by side in one browser still get an answer', async ({ context, request }) => {
  const slug = await aDiscipline(request);
  const sabre = await request.post('/api/disciplines', { data: { name: 'Connections Sabre' } });
  const other = sabre.ok() ? ((await sabre.json()).slug as string) : 'connections-sabre';
  const pages = [];
  for (const path of ['/admin', `/d/${slug}/admin`, `/d/${other}/admin`, '/display/mats', '/plan']) {
    const page = await context.newPage();
    // The old way, the last of these never got a connection to load on.
    await page.goto(path, { timeout: 5000 }).catch(() => {});
    // Its streams open once its first look is in.
    await page.waitForTimeout(1500);
    pages.push(page);
  }
  const status = await pages[0].evaluate(async () => {
    const answer = fetch('/api/event').then((r) => r.status);
    const stuck = new Promise<number>((resolve) => setTimeout(() => resolve(0), 5000));
    return Promise.race([answer, stuck]);
  });
  expect(status, 'a request should be answered, not left waiting for a connection').toBe(200);
});
