import { defineConfig } from 'vite';
import { svelte } from '@sveltejs/vite-plugin-svelte';

// One bundle serves every surface -- the organizer views, the score keeper client and
// /display/* -- with the route deciding what renders. No SvelteKit: there is no
// server-side rendering here, and a Node runtime in the shipped artifact is not
// acceptable while one at build time is (docs/tech-stack.md §6).
// The demo build (issue #88) is the same bundle under a project path on GitHub Pages,
// with VITE_DEMO switching in the wasm adapter. `base` is what makes the assets resolve
// under /porta-di-ferro/; the router strips it back off so no route has to know.
export default defineConfig(({ mode }) => ({
  base: mode === 'demo' ? '/porta-di-ferro/' : '/',
  plugins: [svelte()],
  build: {
    outDir: 'dist',
    emptyOutDir: true,
    // The cold-load budget is measured against this (docs/tech-stack.md §11), so keep
    // the bundle in one place where its size is visible rather than split across many
    // lazily fetched chunks that a phone on congested venue wifi has to chase.
    chunkSizeWarningLimit: 400,
  },
  server: {
    proxy: {
      '/api': 'http://localhost:8080',
    },
  },
  test: {
    environment: 'node',
    include: ['src/**/*.test.ts'],
  },
}));
