import { test, expect, type APIRequestContext } from '@playwright/test';

/**
 * A match scored in the browser, end to end (#166): the score keeper's taps reach the
 * server. And a redraw is not undone by the tablet that scored the previous draw (#160):
 * match ids repeat from draw to draw, so the old log on the device must be set aside, not
 * pushed into the new pairing.
 *
 * The specs share one server, so this one works in a discipline of its own, on a mat of its
 * own added to the hall for it.
 */

async function ok(res: Awaited<ReturnType<APIRequestContext['get']>>) {
  expect(res.ok(), `${res.url()}: ${res.status()} ${await res.text()}`).toBeTruthy();
  return res.json();
}

test('a scored match reaches the server, and a redraw is not undone by the tablet that scored it', async ({ page, request }) => {
  const added = await ok(await request.post('/api/disciplines', { data: { name: `Redraw ${Date.now()}` } }));
  const slug: string = added.slug;
  for (const name of ['Bo', 'Cy', 'Di', 'Ek']) {
    await ok(await request.post(`/api/d/${slug}/competitors`, { data: { name, club: name } }));
  }
  await ok(await request.put(`/api/d/${slug}/tournament`, { data: { mats: 1, minPoolSize: 4, maxPoolSize: 4 } }));

  // Its pool 1 first on a mat nobody else uses.
  const ownMat = async () => {
    const view = await ok(await request.get('/api/mats'));
    const mat = view.mats.length + 1;
    await ok(await request.put('/api/mats', { data: { count: mat } }));
    await ok(await request.patch(`/api/plan/items/${slug}/pool-1`, { data: { mat, index: 0 } }));
    return mat;
  };
  const p1m1 = async () => {
    const state = await ok(await request.get(`/api/d/${slug}/state`));
    const events = await ok(await request.get(`/api/d/${slug}/matches/p1m1/events`));
    return { match: state.pools[0].matches.find((m: { id: string }) => m.id === 'p1m1'), events: events.length as number };
  };

  await ok(await request.post(`/api/d/${slug}/tournament/pools`));
  const mat = await ownMat();

  // Score an exchange on the tablet: 2 to red.
  await page.goto(`/score/${mat}`);
  await page.getByRole('region', { name: /^red competitor/i }).getByRole('button', { name: '2', exact: true }).click();
  await page.getByRole('button', { name: 'CONFIRM EXCHANGE' }).click();
  await expect
    // A push already under way when the exchange is confirmed leaves it for the next
    // flush, five seconds on.
    .poll(async () => (await p1m1()).match.state.red.score, { message: 'the exchange should reach the server', timeout: 15_000 })
    .toBe(2);
  const before = (await p1m1()).match;

  // The organizer draws the pools again: p1m1 is a new pairing, to fence from 0-0.
  await ok(await request.post(`/api/d/${slug}/tournament/pools`));
  const after = (await p1m1()).match;
  expect(after.draw).not.toBe(before.draw);
  expect(after.status).toBe('pending');

  // The tablet opens the mat again, now on the new draw's p1m1.
  const again = await ownMat();
  await page.goto(`/score/${again}`);
  await expect(page.getByRole('button', { name: 'CONFIRM EXCHANGE' })).toBeVisible();
  // Give it time to push whatever it would push: its load and a flush tick.
  await page.waitForTimeout(7000);
  const now = await p1m1();
  expect(now.events, "the old draw's log must not land in the new p1m1").toBe(0);
  expect(now.match.status).toBe('pending');

  // It was set aside for the organizer instead.
  const presence = await ok(await request.get('/api/presence'));
  expect(presence.quarantined.some((q: { discipline: string }) => q.discipline === slug)).toBeTruthy();
});
