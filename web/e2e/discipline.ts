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
