import { defineConfig, devices } from '@playwright/test';
import { tmpdir } from 'node:os';
import { join } from 'node:path';

// The suite writes: it adds competitors, draws pools and rewrites the event. So it runs
// its own server on a port of its own, over a tournament in a fresh temporary folder --
// never the organizer's folder under the home directory, which is the default, and never
// an instance somebody already has running on 8080.
const port = 8097;
const dir = join(tmpdir(), `porta-e2e-${Date.now()}`);

export default defineConfig({
  testDir: './e2e',
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 2 : 0,
  workers: process.env.CI ? 1 : undefined,
  reporter: [
    ['html', { open: 'never' }],
    ['list'],
  ],
  use: {
    baseURL: `http://127.0.0.1:${port}`,
    trace: 'on-first-retry',
    screenshot: 'only-on-failure',
  },
  projects: [
    {
      name: 'chromium',
      use: { ...devices['Desktop Chrome'] },
    },
  ],
  webServer: {
    // Loopback only, so Windows Firewall does not ask about the executable go run builds.
    command: `go run ./cmd/porta -port ${port} -host 127.0.0.1 -no-browser -dir "${dir}"`,
    url: `http://127.0.0.1:${port}/api/state`,
    reuseExistingServer: !process.env.CI,
    cwd: '..',
    timeout: 120 * 1000,
  },
});
