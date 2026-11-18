/**
 * The translation review tool (#168): what a Swedish proofreader needs and sv.ts cannot give
 * them -- English and Swedish side by side, where each string is used, which are new or
 * changed since the last review -- as one HTML page that works from a file.
 *
 *   npm run i18n:review                       writes i18n-review.html and says what is new
 *   npm run i18n:review -- --accept file.json takes a reviewer's export: what they approved
 *                                             goes into i18n-reviewed.json, and what they
 *                                             suggested is printed, sv.ts line by line
 *
 * The build contract is unchanged: every t('…') still needs its Swedish in sv.ts, and the
 * i18n test still fails without it. This only makes reading it practical.
 */
import ts from 'typescript';

/**
 * Every entry of sv.ts, in order, with the section it is under (the `// --- … ---` comments)
 * and its line.
 * @param {string} source the text of sv.ts
 * @returns {{ key: string, sv: string, section: string, line: number }[]}
 */
export function parseSv(source) {
  const file = ts.createSourceFile('sv.ts', source, ts.ScriptTarget.Latest, true);
  const out = [];
  let section = '';
  const visit = (node) => {
    if (ts.isObjectLiteralExpression(node)) {
      for (const p of node.properties) {
        if (!ts.isPropertyAssignment(p)) continue;
        for (const c of ts.getLeadingCommentRanges(source, p.getFullStart()) ?? []) {
          const m = /^\/\/\s*---\s*(.*?)\s*-*\s*$/.exec(source.slice(c.pos, c.end));
          if (m) section = m[1];
        }
        const key = ts.isStringLiteral(p.name) || ts.isIdentifier(p.name) ? p.name.text : null;
        const value = ts.isStringLiteral(p.initializer) || ts.isNoSubstitutionTemplateLiteral(p.initializer) ? p.initializer.text : null;
        if (key === null || value === null) continue;
        out.push({ key, sv: value, section, line: file.getLineAndCharacterOfPosition(p.getStart()).line + 1 });
      }
      return;
    }
    ts.forEachChild(node, visit);
  };
  visit(file);
  return out;
}

/**
 * What kind of thing a string is where it is used, from the line it is on: what changes a
 * translation most is whether it is a button, a label, a placeholder or a message.
 * @param {string} line
 */
export function kindOf(line) {
  if (/placeholder=/.test(line)) return 'placeholder';
  if (/aria-label=|title=/.test(line)) return 'label';
  if (/<button|role="menuitem"|role="tab"/.test(line)) return 'button';
  if (/<h[1-6]/.test(line)) return 'heading';
  if (/class="(warn|err|hint|dim)|errorText|throw /.test(line)) return 'message';
  return 'text';
}

/**
 * Where each key is used: the files that ask t() for it, and what kind of thing it is there.
 * @param {{ path: string, text: string }[]} files the source files, paths relative to web/src
 * @param {string[]} keys
 * @returns {Record<string, { files: string[], kinds: string[] }>}
 */
export function usages(files, keys) {
  const want = new Set(keys);
  const out = {};
  const call = /\bt\(\s*(['"])((?:\\.|(?!\1).)*)\1/g;
  for (const f of files) {
    const lines = f.text.split('\n');
    lines.forEach((line, i) => {
      for (const m of line.matchAll(call)) {
        const key = m[2].replace(/\\(['"\\])/g, '$1');
        if (!want.has(key)) continue;
        const u = (out[key] ??= { files: [], kinds: [] });
        const name = f.path.replace(/^.*[\\/]/, '');
        if (!u.files.includes(name)) u.files.push(name);
        // The element is often on the line above a long string.
        const kind = kindOf(`${lines[i - 1] ?? ''}\n${line}`);
        if (!u.kinds.includes(kind)) u.kinds.push(kind);
      }
    });
  }
  return out;
}

/**
 * Each entry's standing against the last review: "new" (never reviewed), "changed" (the
 * Swedish is not what was approved) or "reviewed".
 * @param {{ key: string, sv: string }[]} entries
 * @param {Record<string, string>} reviewed key → the Swedish that was approved
 */
export function standing(entries, reviewed) {
  const out = {};
  for (const e of entries) {
    out[e.key] = !(e.key in reviewed) ? 'new' : reviewed[e.key] === e.sv ? 'reviewed' : 'changed';
  }
  return out;
}

/**
 * Takes a reviewer's export: what they approved, at the Swedish they approved, goes into the
 * reviewed record; what they suggested instead comes back as changes for sv.ts.
 * @param {Record<string, string>} reviewed
 * @param {{ entries: Record<string, { sv: string, approved?: boolean, suggestion?: string, note?: string }> }} review
 * @param {{ key: string, sv: string, line: number }[]} entries sv.ts as it is now
 */
export function accept(reviewed, review, entries) {
  const now = new Map(entries.map((e) => [e.key, e]));
  const next = { ...reviewed };
  const suggestions = [];
  for (const [key, r] of Object.entries(review.entries ?? {})) {
    const e = now.get(key);
    if (!e) continue;
    if (r.suggestion && r.suggestion.trim() && r.suggestion !== e.sv) {
      suggestions.push({ key, line: e.line, from: e.sv, to: r.suggestion.trim(), note: r.note ?? '' });
    } else if (r.approved && r.sv === e.sv) {
      // Approved as it is now: a Swedish changed since the export is not approved by it.
      next[key] = e.sv;
    }
  }
  return { reviewed: Object.fromEntries(Object.entries(next).sort(([a], [b]) => a.localeCompare(b))), suggestions };
}
