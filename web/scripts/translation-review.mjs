#!/usr/bin/env node
// The translation review tool (#168): see i18n-review.mjs for what it is for.
//
//   npm run i18n:review                        writes i18n-review.html (not committed)
//   npm run i18n:review -- --accept review.json takes a reviewer's export
import { readFileSync, readdirSync, statSync, writeFileSync, existsSync } from 'node:fs';
import { join, relative, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';
import { accept, parseSv, standing, usages } from './i18n-review.mjs';

const web = join(dirname(fileURLToPath(import.meta.url)), '..');
const svPath = join(web, 'src', 'lib', 'sv.ts');
const reviewedPath = join(web, 'i18n-reviewed.json');
const outPath = join(web, 'i18n-review.html');

const entries = parseSv(readFileSync(svPath, 'utf8'));
const reviewed = existsSync(reviewedPath) ? JSON.parse(readFileSync(reviewedPath, 'utf8')) : {};

const at = process.argv.indexOf('--accept');
if (at >= 0) {
  const file = process.argv[at + 1];
  if (!file) throw new Error('--accept needs the file the reviewer exported');
  const result = accept(reviewed, JSON.parse(readFileSync(file, 'utf8')), entries);
  writeFileSync(reviewedPath, JSON.stringify(result.reviewed, null, 2) + '\n');
  const approved = Object.keys(result.reviewed).length - Object.keys(reviewed).length;
  console.log(`${approved} newly approved; i18n-reviewed.json updated.`);
  if (result.suggestions.length > 0) {
    console.log(`\n${result.suggestions.length} suggested changes for src/lib/sv.ts:\n`);
    for (const s of result.suggestions) {
      console.log(`  line ${s.line}  '${s.key}'`);
      console.log(`    - ${s.from}`);
      console.log(`    + ${s.to}`);
      if (s.note) console.log(`    note: ${s.note}`);
    }
  }
  process.exit(0);
}

function sources(dir) {
  const out = [];
  for (const name of readdirSync(dir)) {
    const path = join(dir, name);
    if (statSync(path).isDirectory()) out.push(...sources(path));
    else if (/\.(svelte|ts)$/.test(name) && !/\.test\.ts$/.test(name) && name !== 'sv.ts') {
      out.push({ path: relative(join(web, 'src'), path), text: readFileSync(path, 'utf8') });
    }
  }
  return out;
}

const used = usages(sources(join(web, 'src')), entries.map((e) => e.key));
const stand = standing(entries, reviewed);
const rows = entries.map((e) => ({ ...e, ...(used[e.key] ?? { files: [], kinds: [] }), status: stand[e.key] }));
const count = (s) => rows.filter((r) => r.status === s).length;

// Inlined, and closed off so a string in sv.ts can never end the script element.
const data = JSON.stringify(rows).replace(/</g, '\\u003c');
writeFileSync(outPath, page(data));
console.log(`${rows.length} translations: ${count('new')} new, ${count('changed')} changed, ${count('reviewed')} reviewed.`);
console.log(`Open ${relative(process.cwd(), outPath)} in a browser, or send it to the reviewer.`);

function page(data) {
  return `<!doctype html>
<html lang="sv">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Porta di Ferro · översättningsgranskning</title>
<style>
  :root { color-scheme: light dark; --line: #8884; --dim: #888; --new: #c77b00; --changed: #b03a2e; --ok: #2e7d4f; }
  body { font: 15px/1.45 system-ui, sans-serif; margin: 0; padding: 1rem; max-width: 1200px; margin-inline: auto; }
  header { position: sticky; top: 0; background: Canvas; padding: 0.5rem 0; border-bottom: 1px solid var(--line); z-index: 1; }
  h1 { font-size: 1.2rem; margin: 0 0 0.4rem; }
  .bar { display: flex; flex-wrap: wrap; gap: 0.5rem 1rem; align-items: center; }
  .bar input[type=search] { flex: 1 1 14rem; padding: 0.35rem 0.5rem; }
  h2 { font-size: 1rem; margin: 1.4rem 0 0.4rem; }
  h2::first-letter { text-transform: uppercase; }
  table { width: 100%; border-collapse: collapse; }
  td { vertical-align: top; padding: 0.45rem 0.4rem; border-top: 1px solid var(--line); }
  td.en { width: 32%; }
  td.sv { width: 38%; }
  textarea { width: 100%; box-sizing: border-box; font: inherit; min-height: 2.6em; }
  .where, .hint { color: var(--dim); font-size: 0.82rem; }
  .badge { font-size: 0.75rem; padding: 0.05rem 0.4rem; border-radius: 0.6rem; color: #fff; }
  .badge.new { background: var(--new); } .badge.changed { background: var(--changed); } .badge.reviewed { background: var(--ok); }
  tr.done td { opacity: 0.6; }
  .ph { font-family: ui-monospace, monospace; background: #8882; border-radius: 3px; padding: 0 2px; }
</style>
</head>
<body>
<header>
  <h1>Översättningsgranskning <span class="hint">· Translation review</span></h1>
  <p class="hint">Engelska till vänster, svenska till höger. Bocka i <b>Godkänd</b> när svenskan är bra. Skriv ett bättre förslag i rutan om den inte är det, och en anteckning om något behöver förklaras. {namn} i klamrar fylls i av programmet och ska stå kvar. Allt sparas i den här webbläsaren; tryck <b>Exportera</b> när du är klar och skicka filen till den som underhåller programmet.</p>
  <div class="bar">
    <input type="search" id="q" placeholder="Sök på engelska eller svenska">
    <label><input type="radio" name="show" value="todo" checked> Nya och ändrade</label>
    <label><input type="radio" name="show" value="open"> Inte godkända</label>
    <label><input type="radio" name="show" value="all"> Alla</label>
    <span id="progress" class="hint"></span>
    <button id="export">Exportera granskningen</button>
  </div>
</header>
<main id="list"></main>
<script>
const rows = ${data};
const KEY = 'porta-i18n-review';
let state = {};
try { state = JSON.parse(localStorage.getItem(KEY) || '{}'); } catch (e) {}
const save = () => { try { localStorage.setItem(KEY, JSON.stringify(state)); } catch (e) {} };
const kinds = { button: 'knapp', label: 'etikett', placeholder: 'platshållare', heading: 'rubrik', message: 'meddelande', text: 'text' };
const statusName = { new: 'ny', changed: 'ändrad', reviewed: 'granskad' };
// A review is of the Swedish as it was shown: a newer Swedish starts over.
const own = (r) => { const s = state[r.key]; return s && s.sv === r.sv ? s : null; };
const esc = (s) => s.replace(/[&<>"]/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;' })[c]);
const marks = (s) => esc(s).replace(/\\{[a-zA-Z]+\\}/g, (m) => '<span class="ph">' + m + '</span>');

function render() {
  const q = document.getElementById('q').value.trim().toLowerCase();
  const show = document.querySelector('input[name=show]:checked').value;
  const list = document.getElementById('list');
  list.textContent = '';
  let section = null, table = null, done = 0;
  for (const r of rows) {
    const mine = own(r);
    if (mine && mine.approved) done++;
    if (q && !r.key.toLowerCase().includes(q) && !r.sv.toLowerCase().includes(q)) continue;
    if (show === 'todo' && r.status === 'reviewed') continue;
    if (show === 'open' && mine && mine.approved) continue;
    if (r.section !== section) {
      section = r.section;
      const h = document.createElement('h2'); h.textContent = section || '—'; list.append(h);
      table = document.createElement('table'); list.append(table);
    }
    const tr = document.createElement('tr');
    if (mine && mine.approved) tr.className = 'done';
    tr.innerHTML =
      '<td class="en">' + marks(r.key) + '<div class="where">' +
        (r.files.length ? esc(r.files.join(', ')) + ' · ' + r.kinds.map((k) => kinds[k] || k).join(', ') : 'används via en variabel, t.ex. en färg eller roll') +
        ' · rad ' + r.line + '</div></td>' +
      '<td class="sv"><div>' + marks(r.sv) + ' <span class="badge ' + r.status + '">' + statusName[r.status] + '</span></div>' +
        '<textarea placeholder="Bättre förslag (lämna tomt om den är bra)">' + esc((mine && mine.suggestion) || '') + '</textarea></td>' +
      '<td><label><input type="checkbox"' + (mine && mine.approved ? ' checked' : '') + '> Godkänd</label>' +
        '<textarea placeholder="Anteckning">' + esc((mine && mine.note) || '') + '</textarea></td>';
    const [suggestion, note] = tr.querySelectorAll('textarea');
    const box = tr.querySelector('input[type=checkbox]');
    const update = () => {
      state[r.key] = { sv: r.sv, approved: box.checked, suggestion: suggestion.value, note: note.value };
      tr.className = box.checked ? 'done' : '';
      save(); progress();
    };
    box.addEventListener('change', update);
    suggestion.addEventListener('input', update);
    note.addEventListener('input', update);
    table.append(tr);
  }
  progress();
}
function progress() {
  const done = rows.filter((r) => { const m = own(r); return m && m.approved; }).length;
  document.getElementById('progress').textContent = done + ' av ' + rows.length + ' godkända';
}
document.getElementById('q').addEventListener('input', render);
document.querySelectorAll('input[name=show]').forEach((el) => el.addEventListener('change', render));
document.getElementById('export').addEventListener('click', () => {
  const entries = {};
  for (const r of rows) { const m = own(r); if (m && (m.approved || m.suggestion || m.note)) entries[r.key] = m; }
  const blob = new Blob([JSON.stringify({ exported: new Date().toISOString(), entries }, null, 2)], { type: 'application/json' });
  const a = document.createElement('a');
  a.href = URL.createObjectURL(blob);
  a.download = 'translation-review.json';
  a.click();
});
render();
</script>
</body>
</html>
`;
}
