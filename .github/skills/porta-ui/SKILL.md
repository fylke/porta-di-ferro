---
name: porta-ui
description: "Use when building, changing, reviewing, or debugging user-facing UI in this repository: Svelte pages and components, organizer workflows, score keeping, displays, responsive or touch behavior, translations, and browser interactions."
---

# Porta di Ferro UI Work

Use this workflow for user-facing changes in `web/`, including the standalone participant signup page. The goal is a coherent, dependable interface across the organizer's computer, venue tablets and phones, and spectator screens.

## Before Changing UI

1. Read the relevant constraints in [AGENTS.md](../../../AGENTS.md) and product decisions in [docs/design.md](../../../docs/design.md). Read [docs/architecture.md](../../../docs/architecture.md) when the screen crosses event and discipline boundaries.
2. Identify the surface and its user: organizer/admin, score keeper, display, or offline participant signup. Inspect a neighboring page/component and its tests; reuse the established layout, controls, CSS variables, and interaction patterns.
3. Keep the change local. Do not redesign shared styling or neighboring flows to solve a page-specific issue. Preserve the existing visual language in [web/src/app.css](../../../web/src/app.css), including the score keeper's red/blue identity semantics.

## Repository UI Rules

- Every screen must provide all user-facing content in every supported language: English and Swedish. This includes headings, labels, placeholders, help, validation, errors, empty/loading/success states, document titles, accessible names, and guided-tour content, not only the main page copy. When touching a screen, check all of its states and do not leave untranslated strings as an assumed fallback.
- In the Svelte app, every string a person reads goes through `t()` from `web/src/lib/i18n.svelte.ts`. Add the matching English-keyed entry to `web/src/lib/sv.ts` in the same change. Keep API values and internal identifiers untranslated.
- Standalone pages that cannot import the Svelte bundle need their own small, self-contained translations and a usable language choice or detection mechanism. Keep the document's `lang` in sync with the selected language. The participant signup page is currently English-only; treat it as a localization gap to address, not as precedent for shipping English-only screens. Preserve its `file://`, no-build, no-network contract while adding translations.
- Keep discipline context intact: use `dhref()` for links within a discipline, call the page's `api` or `apiIn(slug)` for its API, and namespace device-persisted discipline data with the helpers in `web/src/lib/paths.ts`.
- Venue clients use ordinary HTTP on a LAN, not a secure context. Do not rely on `crypto.randomUUID`, clipboard, wake lock, or other secure-context-only browser APIs without a supported fallback; prefer existing helpers.
- Make the important states visible: loading, empty, unavailable/offline, pending, success, and failure as appropriate to the workflow. Do not leave an action looking successful when its request failed.
- Use semantic controls with accessible names and keyboard behavior. Make touch interactions reliable on tablets and phones; ensure labels, translated text, and controls fit without horizontal overflow at narrow widths.
- Keep red and blue tied to competitor identity, not generic status. Do not use color alone to communicate a decision or state.
- `web/signup/signup.html` is intentionally a single self-contained file that works from `file://`, without a build step or secure context. Keep that contract; do not move its logic into the Svelte app or require network assets. Update `docs/proposals/offline-signup.md` when changing its file format or import behavior.

## Review for Known UI Failure Patterns

Use [#142](https://github.com/fylke/porta-di-ferro/issues/142), [#149](https://github.com/fylke/porta-di-ferro/issues/149), and [#153](https://github.com/fylke/porta-di-ferro/issues/153) as examples of observed usability problems, not as automatic product requirements. Check the proposed change for these patterns:

- **Too much, repeated, or mistimed information:** organize around the user's current task and event stage. Show a fact once, defer infrequent setup/actions until relevant, and keep planning distinct from live-day monitoring.
- **Unclear language or controls:** use the organizer's words, not implementation terms. Explain what a value means, its units, defaults, and effect at the point of decision. Put shared units in introductory copy rather than repeating them on every field. Do not make hover-only text carry essential meaning; it must also be available on focus and touch.
- **Warnings without a path forward:** when the UI flags a missing or invalid condition, offer a nearby way to fix it or clearly say where to go. Do not report a problem the current workflow gives no way to address.
- **Actions that feel inert or risky:** label a button with its outcome, place it with the fields or preview it affects, and show clear confirmation or updated results. For consequential multi-item changes, prefer a visible draft with Apply and Discard over silently committing each adjustment.
- **Unhelpful timing and defaults:** ask for a decision when the user has enough information to make it; defer or recommend choices that depend on later inputs. Make required setup steps and completion criteria explicit.
- **Inconsistent or incomplete guidance:** keep control styles and sizing consistent within a surface. Tours and instructions should include useful examples and describe the real workflow without assuming a particular device such as a tablet.

Before calling a UI flow done, try its first-use path and a realistic incomplete or empty state. Ask whether a person can tell what they are looking at, what is expected next, what an action will change, and whether it worked.

## Verify the Change

1. Add or update a focused test for changed behavior. Prefer existing Vitest tests for logic and Playwright tests for browser workflows; test the actual user action and its visible result in both supported languages, including relevant validation and fallback states.
2. From `web/`, run `npm run check` and `npm test`. Run the relevant Playwright coverage with `npm run test:e2e` when changing a user workflow or browser behavior.
3. For layout or interaction changes, inspect the affected page at a narrow phone/tablet size and a desktop size. Check for clipped/overlapping content, horizontal scrolling, unusable touch controls, and console errors. For signup changes, retain the existing `file://` and JavaScript-disabled checks where relevant.
4. Report any browser/device condition you could not verify; do not treat a successful typecheck as proof that the interaction works.