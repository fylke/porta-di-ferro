import type { APIRequestContext } from '@playwright/test';

/**
 * The discipline these specs work in. A new event has none (#149), and the specs share
 * one server, so the first to ask makes it and the rest find it -- including one asking at
 * the same moment, whose add is refused as a second of the same name.
 */
export async function aDiscipline(request: APIRequestContext): Promise<string> {
  const first = async () => ((await (await request.get('/api/event')).json()).disciplines[0]?.slug as string | undefined);
  const had = await first();
  if (had) return had;
  const res = await request.post('/api/disciplines', { data: { name: 'Open Steel Longsword' } });
  if (res.ok()) return (await res.json()).slug;
  const now = await first();
  if (!now) throw new Error(`no discipline to test in: ${res.status()} ${await res.text()}`);
  return now;
}

async function ok(res: Awaited<ReturnType<APIRequestContext['get']>>) {
  if (!res.ok()) throw new Error(`${res.url()}: ${res.status()} ${await res.text()}`);
  return res.json();
}

/**
 * A discipline of the spec's own, four entered and its pools drawn, for a spec that scores:
 * the specs share one server, and a match somebody else's spec has touched is no test.
 */
export async function aPoolOfOurOwn(request: APIRequestContext, name: string): Promise<string> {
  const slug: string = (await ok(await request.post('/api/disciplines', { data: { name: `${name} ${Date.now()}` } }))).slug;
  for (const who of ['Bo', 'Cy', 'Di', 'Ek']) {
    await ok(await request.post(`/api/d/${slug}/competitors`, { data: { name: who, club: who } }));
  }
  await ok(await request.put(`/api/d/${slug}/tournament`, { data: { mats: 1, minPoolSize: 4, maxPoolSize: 4 } }));
  await ok(await request.post(`/api/d/${slug}/tournament/pools`));
  // Beside everybody else's, in the first block of the day: a discipline added later runs
  // in a block of its own, and its mat waits for the blocks before it to finish (#169).
  await ok(await request.put('/api/plan/sessions', { data: { [slug]: 1 } }));
  return slug;
}

/**
 * Puts a discipline's pool 1 alone on a mat added to the hall for it, and says which. Specs
 * run side by side, so two can add the same mat: whichever finds another's item on it
 * tries the next.
 */
export async function onAMatOfItsOwn(request: APIRequestContext, slug: string): Promise<number> {
  for (let tries = 0; tries < 5; tries++) {
    const view = await ok(await request.get('/api/mats'));
    const mat = view.mats.length + 1;
    await ok(await request.put('/api/mats', { data: { count: Math.max(mat, (await ok(await request.get('/api/mats'))).mats.length) } }));
    await ok(await request.patch(`/api/plan/items/${slug}/pool-1`, { data: { mat, index: 0 } }));
    const now = await ok(await request.get('/api/mats'));
    const on = (now.items as { id: string; mat: number }[]).filter((it) => it.mat === mat);
    if (on.length === 1 && on[0].id === `${slug}/pool-1`) return mat;
  }
  throw new Error(`no mat of its own for ${slug}`);
}
