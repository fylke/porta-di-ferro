import { test, expect } from '@playwright/test';
import { aPoolOfOurOwn, onAMatOfItsOwn } from './discipline';

test.describe('Offline & Route Fallthrough', () => {
  // REQ: NFR_OFFLINE_01
  test('direct navigation to SPA routes fallthrough to index.html', async ({ page }) => {
    // Direct load of deep route
    await page.goto('/score/1');
    await expect(page.url()).toContain('/score/1');

    await page.goto('/display/mat/2');
    await expect(page.url()).toContain('/display/mat/2');
  });

  // The score keeper keeps scoring when the wifi drops, and what was scored reaches the
  // server when it is back (#166: this used to assert only that the URL had not changed).
  // REQ: FR_OFFLINE_01, NFR_PERF_01
  test('an exchange scored offline reaches the server when the network is back', async ({ page, context, request }) => {
    const slug = await aPoolOfOurOwn(request, 'Offline');
    const mat = await onAMatOfItsOwn(request, slug);
    const redScore = async () => {
      const state = await (await request.get(`/api/d/${slug}/state`)).json();
      return state.pools[0].matches[0].state.red.score as number;
    };

    await page.goto(`/score/${mat}`);
    await expect(page.getByRole('button', { name: 'CONFIRM EXCHANGE' })).toBeVisible();
    await context.setOffline(true);

    const red = page.getByRole('region', { name: /^red competitor/i });
    await red.getByRole('button', { name: '2', exact: true }).click();
    await page.getByRole('button', { name: 'CONFIRM EXCHANGE' }).click();
    // Scored on the tablet, with nobody to tell.
    await expect(red.getByLabel('score')).toHaveText('2');
    expect(await redScore()).toBe(0);

    await context.setOffline(false);
    await expect.poll(redScore, { message: 'the offline exchange should reach the server', timeout: 15_000 }).toBe(2);
  });
});
