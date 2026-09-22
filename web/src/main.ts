import './app.css';
import { mount } from 'svelte';
import App from './App.svelte';
import { lang } from './lib/i18n.svelte';
import { navigate } from './router.svelte';

document.documentElement.lang = lang.current;

/**
 * The demo build (issue #88): the same application, with cmd/demo-wasm where the Go
 * server would be. `npm run build:demo` sets it; a normal build leaves it unset and Vite
 * drops everything below, wasm loader and all, from the bundle.
 */
const demo = import.meta.env.VITE_DEMO === 'true';

async function boot() {
  if (demo) {
    // Before the app mounts, so the first request the client makes already has something
    // to answer it. A failure here has to be said out loud: an application whose every
    // call 404s looks broken rather than unloaded.
    const { startDemo } = await import('./demo/adapter');
    try {
      await startDemo(navigate);
    } catch (e) {
      document.getElementById('app')!.innerHTML =
        `<main style="max-width:34rem;margin:4rem auto;padding:0 1.5rem;line-height:1.6">
           <h1>The demo did not load</h1>
           <p>${e instanceof Error ? e.message : String(e)}</p>
           <p><a href="https://github.com/fylke/porta-di-ferro">The project on GitHub</a></p>
         </main>`;
      return;
    }
  } else if ('serviceWorker' in navigator) {
    // The app shell cache. Registered here rather than in index.html so a build without a
    // service worker is still a working application.
    //
    // Not in the demo: its scope would be the whole of github.io rather than the project
    // path, and there is no offline story to tell when the server is a wasm module that
    // came down with the page.
    window.addEventListener('load', () => {
      void navigator.serviceWorker.register('/sw.js').catch(() => {
        // Unsupported or blocked. The application works; it just will not survive a
        // network drop on a device that has not loaded it before.
      });
    });
  }

  mount(App, { target: document.getElementById('app')! });
}

void boot();
