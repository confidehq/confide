# Form Customizations, Themes, Completion Message, Redirect on Submit

**Status:** Planned

---

## Overview

This document covers four related features that give form owners control over the post-submit experience and the public form's visual appearance:

1. **Completion message** — customizable text shown after submission (all layouts, not just convo)
2. **Redirect on submit** — optionally redirect to a URL instead of showing the completion message
3. **Form themes** — four named visual presets for the public-facing form
4. **Form customizations** — per-form overrides for accent color, background color, font, and logo

All new config lives in the encrypted `BuilderSchema` JSON blob. No database migration is needed.

---

## Data Model

### New types in `web/src/lib/types/builder.ts`

```ts
export type FormTheme = 'default' | 'minimal' | 'dark' | 'warm';
export type FormFont  = 'system' | 'serif' | 'mono';

export interface FormCustomization {
  accentColor?:     string;   // hex — overrides --color-form-primary etc.
  backgroundColor?: string;   // hex — overrides --color-form-bg
  font?:            FormFont;
  logoUrl?:         string;   // image URL displayed above the form title
}
```

New optional fields added to `BuilderSchema`:

```ts
theme?:             FormTheme;
customization?:     FormCustomization;
submitRedirectUrl?: string;   // takes precedence over completion message when set
```

`TranslationMap.convoCompletionMessage` is unchanged — the public form page already uses this field for all layout types in the submitted state. Only the settings UI guard changes.

---

## Theme Presets

Defined in a new file `web/src/lib/form-themes.ts`.

| Theme | Background | Primary/Accent |
|-------|-----------|----------------|
| `default` | `#ffffff` | `#1d4ed8` (blue) — existing defaults, no overrides |
| `minimal` | `#f5f5f5` | `#404040` (dark grey) |
| `dark` | `#111827` | `#60a5fa` (light blue) |
| `warm` | `#fffbf0` | `#b45309` (amber) |

Presets are stored as `Record<FormTheme, Record<string, string>>` mapping CSS variable names to values. The `default` preset is an empty map (existing global CSS vars apply).

Two helper exports:

- `buildThemeCssVars(schema: BuilderSchema): string` — merges the preset's CSS vars with any per-form `customization` overrides (accent/bg color), returns an inline `style` attribute string.
- `buildFontStack(font: FormFont | undefined): string` — returns the CSS `font-family` value for the chosen font option.

---

## Settings Panel (`FormSettingsPanel.svelte`)

Four new sections added to the settings panel:

### Completion message (all layouts)

Remove the `{#if isConvo}` guard around the "Completion message" textarea. The "Allow edit after submit" toggle stays gated on `isConvo` since that option is convo-specific.

### Redirect on submit

A toggle + `<input type="url">` below the completion message. When a URL is set, a note reads: "Redirect takes precedence over the completion message."

Calls `store.setSubmitRedirectUrl(url | null)`.

### Theme selector

Four small preview swatches (colored cards/pills showing the primary and background color) labeled Default, Minimal, Dark, Warm. Clicking one calls `store.setTheme(theme)`. The active theme is highlighted.

### Customizations

Below the theme selector, a collapsible or always-visible section with:

- **Accent color** — `<input type="color">` → `store.setCustomization({ accentColor })`
- **Background color** — `<input type="color">` → `store.setCustomization({ backgroundColor })`
- **Font** — `<select>` with System / Serif / Monospace → `store.setCustomization({ font })`
- **Logo URL** — `<input type="url" placeholder="https://…">` → `store.setCustomization({ logoUrl })`

---

## Builder Store (`builder.svelte.ts`)

Three new actions added to the `BuilderStore` interface and implementation:

```ts
setTheme(theme: FormTheme | undefined): void;
setCustomization(patch: Partial<FormCustomization>): void;
setSubmitRedirectUrl(url: string | null): void;
```

Each mutates `schema` and calls `markDirty()`. Changes are auto-saved by the existing 2-second debounce effect.

---

## Public Form Page (`f/[id]/+page.svelte`)

### Theme application

After schema loads, derive `buildThemeCssVars(schema)` and `buildFontStack(schema.customization?.font)`. Apply them as `style` attributes on the renderer wrapper element so the CSS vars cascade into both `ScrollRenderer` and `StepsRenderer`.

For dark themes, also set `background` and `color` on `<body>` to avoid a flash of white background before the renderer mounts.

### Redirect on submit

```ts
function handleSubmitted() {
  const url = schema?.submitRedirectUrl;
  if (url) { window.location.href = url; return; }
  formState = 'submitted';
}
```

### Logo

Pass `schema.customization?.logoUrl` as a `logoUrl` prop to both renderer components.

---

## Renderers (`ScrollRenderer.svelte`, `StepsRenderer.svelte`)

Add `logoUrl?: string` to each renderer's `Props` interface. When set, render an image above the form title:

```svelte
{#if logoUrl}
  <img src={logoUrl} alt="" class="max-h-16 mb-4 object-contain" />
{/if}
```

---

## Preview (`FormPreview.svelte`)

Apply `buildThemeCssVars(schema)` and `buildFontStack(...)` as `style` on the preview wrapper div so the preview in the builder reflects the selected theme and customizations in real time.

---

## Files Changed

| File | Change |
|------|--------|
| `web/src/lib/types/builder.ts` | Add `FormTheme`, `FormFont`, `FormCustomization`; extend `BuilderSchema` |
| `web/src/lib/form-themes.ts` | New — theme preset map + CSS var builder helpers |
| `web/src/lib/stores/builder.svelte.ts` | Add `setTheme`, `setCustomization`, `setSubmitRedirectUrl` |
| `web/src/lib/components/builder/FormSettingsPanel.svelte` | Completion msg for all layouts; redirect; theme selector; customization inputs |
| `web/src/routes/f/[id]/+page.svelte` | Apply theme vars; handle redirect; pass `logoUrl` |
| `web/src/lib/components/form/ScrollRenderer.svelte` | Accept and render `logoUrl` |
| `web/src/lib/components/form/StepsRenderer.svelte` | Accept and render `logoUrl` |
| `web/src/lib/components/form/FormPreview.svelte` | Apply theme vars on preview wrapper |
