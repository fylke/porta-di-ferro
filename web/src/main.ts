import './app.css';
import { mount } from 'svelte';
import App from './App.svelte';

// The app shell cache. Registered here rather than in index.html so a build without a
// service worker is still a working application.
if ('serviceWorker' in navigator) {
  window.addEventListener('load', () => {
    void navigator.serviceWorker.register('/sw.js').catch(() => {
      // Unsupported or blocked. The application works; it just will not survive a network
      // drop on a device that has not loaded it before.
    });
  });
}

export default mount(App, { target: document.getElementById('app')! });
