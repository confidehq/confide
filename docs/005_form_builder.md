# Phase 5 — Form Builder + Schema Versioning

**Status:** Planning
**Exit criterion:** Creator can build, translate, preview, publish, and share a multi-language form in under 3 minutes without reading documentation.

---

## DQ4 — Schema versioning decision

**Decision: Adopt the snapshot model.**

Each `PUT /api/forms/{id}` inserts an immutable row into a new `form_schema_versions` table **and** updates the live `encrypted_schema` on the `forms` table. The `schema_version` counter on `forms` is the canonical version number — it monotonically increments and matches the `version` column in `form_schema_versions`.

**Why snapshots matter for Phase 6:** The response viewer must render each response against the schema it was submitted under. Without immutable snapshots, a schema edit silently changes how old responses appear.

**What this means for Phase 5:**
- `CreateForm` inserts version 1 into `form_schema_versions` atomically.
- `UpdateFormSchema` increments `schema_version`, updates `encrypted_schema` on `forms`, and inserts the new blob into `form_schema_versions`, all in one transaction.
- New endpoint: `GET /api/forms/{id}/schema-versions/{version}` — returns the specific versioned blob (authenticated, owner-only). Phase 6 uses this for the response viewer. Phase 5 wires it up but doesn't render it.

**Schema version snapshots store only the owner-encrypted schema** (`formKey`-encrypted), not the render-encrypted version. The render-encrypted schema is always the current published one. Past render keys are not stored — once rotated, old links are dead.

---

## Step 1 — DB migration (0007)

### `migrations/0007_schema_versions.up.sql`

```sql
CREATE TABLE form_schema_versions (
  form_id          TEXT     NOT NULL REFERENCES forms(id) ON DELETE CASCADE,
  version          INTEGER  NOT NULL,
  encrypted_schema BYTEA    NOT NULL,
  created_at       DATE     NOT NULL,
  PRIMARY KEY (form_id, version)
);
```

### `migrations/0007_schema_versions.down.sql`

```sql
DROP TABLE IF EXISTS form_schema_versions;
```

---

## Step 2 — SQLC queries

Add to `internal/db/queries/forms.sql`:

```sql
-- name: InsertSchemaVersion :exec
INSERT INTO form_schema_versions (form_id, version, encrypted_schema, created_at)
VALUES ($1, $2, $3, CURRENT_DATE);

-- name: GetSchemaVersion :one
SELECT encrypted_schema FROM form_schema_versions
WHERE form_id = $1 AND version = $2;

-- name: ListSchemaVersions :many
SELECT version, created_at FROM form_schema_versions
WHERE form_id = $1 ORDER BY version DESC;
```

Run `sqlc generate` after writing.

---

## Step 3 — Backend: snapshot integration

### `internal/forms/service.go`

**`CreateForm`** — after inserting the form row, insert version 1 into `form_schema_versions` in the same transaction:

```go
// In CreateForm, after INSERT INTO forms:
if err := q.InsertSchemaVersion(ctx, db.InsertSchemaVersionParams{
    FormID:          id,
    Version:         1,
    EncryptedSchema: req.EncryptedSchema,
}); err != nil {
    return "", err
}
```

Wrap both inserts in `pool.BeginTx` / commit.

**`UpdateFormSchema`** — existing query already increments `schema_version` and returns the new version. Add a second insert into `form_schema_versions` in the same transaction:

```go
newVersion, err := q.UpdateFormSchema(ctx, ...)
// then:
q.InsertSchemaVersion(ctx, db.InsertSchemaVersionParams{
    FormID:          formID,
    Version:         newVersion,
    EncryptedSchema: req.EncryptedSchema,
})
```

Add to `service.go`:

```go
func (s *Service) GetSchemaVersion(ctx context.Context, accountID, formID string, version int32) ([]byte, error)
```

Verifies ownership via `GetFormByOwner`, then calls `GetSchemaVersion` query.

### `internal/forms/handler.go`

Add one new authenticated route:

```
GET /api/forms/{id}/schema-versions/{version}
```

Response:
```json
{ "encryptedSchema": "<base64>", "version": 3 }
```

Returns 404 for wrong owner or missing version.

### `internal/server/server.go`

Wire the new handler into the existing authenticated forms router. No structural changes needed — it mounts alongside the existing form routes.

---

## Step 4 — Frontend: strongly-typed field types

### `web/src/lib/types/builder.ts`

Define all 8 field configs as a discriminated union so the builder components get full type inference:

```typescript
export type FieldType =
  | 'short_text'
  | 'long_text'
  | 'multiple_choice'
  | 'checkboxes'
  | 'dropdown'
  | 'date_time'
  | 'rating'
  | 'section_break';

export interface ShortTextConfig  { maxLength?: number }
export interface LongTextConfig   { maxLength?: number; minRows?: number }
export interface ChoiceOption     { id: string; order: number }
export interface MultipleChoiceConfig { options: ChoiceOption[]; allowOther?: boolean }
export interface CheckboxesConfig { options: ChoiceOption[]; minSelect?: number; maxSelect?: number }
export interface DropdownConfig   { options: ChoiceOption[]; searchable?: boolean }
export interface DateTimeConfig   { mode: 'date' | 'time' | 'datetime'; min?: string; max?: string }
export interface RatingConfig     { scale: 5 | 10; shape: 'star' | 'number' }
export interface SectionBreakConfig { /* no config */ }

export type FieldConfig =
  | ShortTextConfig | LongTextConfig | MultipleChoiceConfig
  | CheckboxesConfig | DropdownConfig | DateTimeConfig
  | RatingConfig | SectionBreakConfig;

export interface BuilderField {
  id: string;
  type: FieldType;
  required: boolean;
  order: number;
  config: FieldConfig;
}

export interface TranslationMap {
  formTitle: string;
  formDescription: string;
  convoCompletionMessage?: string;
  fields: Record<string, {
    label: string;
    helpText?: string;
    placeholder?: string;
    options?: string[];  // parallel to config.options array, indexed by order
  }>;
}

export interface BuilderSchema {
  version: number;
  defaultLocale: string;
  locales: string[];
  layout: 'scroll' | 'steps' | 'convo';
  convoAllowEdit?: boolean;
  fields: BuilderField[];
  translations: Record<string, TranslationMap>;
}
```

### Update `web/src/lib/types/crypto.ts`

Import `BuilderSchema` and re-export it as `FormSchema` so the crypto layer is still agnostic:

```typescript
export type { BuilderSchema as FormSchema } from './builder.ts';
```

This is a non-breaking change — the crypto layer accepts `FormSchema` as JSON-serialisable and encrypts it opaquely.

---

## Step 5 — Builder store

### `web/src/lib/stores/builder.svelte.ts`

Runes-based store holding all mutable builder state. Manages the schema, tracks dirty state, debounces saves, and holds the active locale and selected field.

```typescript
// State
let schema = $state<BuilderSchema>(emptySchema());
let saving = $state(false);
let lastSaved = $state<Date | null>(null);
let dirty = $state(false);
let activeLocale = $state('en');
let selectedFieldId = $state<string | null>(null);
let mode = $state<'edit' | 'preview'>('edit');

// Derived
const selectedField = $derived(
  schema.fields.find(f => f.id === selectedFieldId) ?? null
);
const activeTranslation = $derived(
  schema.translations[activeLocale] ?? schema.translations[schema.defaultLocale]
);

// Actions
function addField(type: FieldType): void        // append to end, select it
function removeField(id: string): void
function reorderFields(newOrder: BuilderField[]): void  // from dnd-action
function updateField(id: string, patch: Partial<BuilderField>): void
function updateFieldConfig(id: string, patch: Partial<FieldConfig>): void
function updateTranslation(fieldId: string | null, key: string, value: string): void
function addLocale(locale: string): void
function removeLocale(locale: string): void
function setLayout(layout: BuilderSchema['layout']): void

// Save (called by debounce and explicit publish)
async function save(masterKey: CryptoKey, formId: string): Promise<void>
```

Debounce: `$effect` watching `schema` — after 2 seconds of inactivity, call `save()`. On mount, load schema via `getForm()`.

`emptySchema()` returns a minimal valid schema: `version:1`, `defaultLocale:'en'`, `locales:['en']`, `layout:'scroll'`, empty `fields`, and a blank `TranslationMap` for `'en'`.

---

## Step 6 — New form route

### `web/src/routes/(app)/forms/new/+page.ts`

```typescript
export const ssr = false;
export const prerender = false;
```

### `web/src/routes/(app)/forms/new/+page.svelte`

No-content page. On mount:
1. Reads `masterKey` from auth store — if null, redirect to `/login`.
2. Calls `createForm(masterKey, emptySchema())`.
3. Redirects to `/forms/{formId}/edit`.
4. Shows a "Creating form…" spinner while in-flight. No form content displayed.

On error: show message, offer "Go back to forms" link.

---

## Step 7 — Builder route

### `web/src/routes/(app)/forms/[id]/edit/+page.ts`

```typescript
export const ssr = false;
export const prerender = false;
```

### `web/src/routes/(app)/forms/[id]/edit/+page.svelte`

Top-level builder page. Orchestrates the three-panel layout and toolbar. Owns the builder store instance (passed as context to child components).

**Layout (CSS Grid):**
```
[Toolbar — full width]
[FieldPalette | FieldCanvas | PropertiesPanel]
```
Proportions: 240px | 1fr | 320px. On viewport < 1024px: properties panel collapses into a bottom drawer.

**Toolbar contents (left to right):**
- Form title input (bound to `activeTranslation.formTitle`)
- Layout selector: scroll / steps / convo (segmented button)
- Locale switcher dropdown (shows available locales + "Add language" option)
- `|` separator
- Save indicator: "Saving…" / "Saved" / "Unsaved changes"
- Preview toggle button
- Publish button (primary, disabled while saving)

**On load:**
1. `getForm(masterKey, formId)` — decrypt schema, hydrate store.
2. If form not found → redirect to `/forms`.

**Publish flow** (triggered by Publish button):
1. Flush any pending debounced save first.
2. Generate new `renderKey` = `crypto.subtle.generateKey('AES-GCM', 256)`.
3. Encrypt current schema with `renderKey` → `renderEncryptedSchema`.
4. Re-derive `publicFormKey` from `formKey` (same as in `createForm`).
5. `PUT /api/forms/{id}` with `encryptedSchema`, `renderEncryptedSchema`, `publicFormKey`.
6. Export `renderKey` as raw bytes → base64url.
7. Compose share URL: `https://<host>/f/{id}#rk=<renderKey_b64url>`.
8. Open a modal: "Your form is live. Share this link:" with copy button and a "Rotate key" option.

**renderKey rotation** (from the share modal):
- Runs the same publish flow, generating a fresh `renderKey`.
- Old share URLs immediately stop working (server stores only the latest `renderEncryptedSchema`).

---

## Step 8 — Builder subcomponents

### `web/src/lib/components/builder/FieldPalette.svelte`

Left panel. Shows 8 field type tiles, each with a label and an icon:

| Type | Label | Icon (text/emoji) |
|------|-------|---|
| `short_text` | Short text | `Aa` |
| `long_text` | Long text | `¶` |
| `multiple_choice` | Multiple choice | `◉` |
| `checkboxes` | Checkboxes | `☑` |
| `dropdown` | Dropdown | `▾` |
| `date_time` | Date / time | `📅` |
| `rating` | Rating | `★` |
| `section_break` | Section break | `—` |

Clicking a tile calls `addField(type)`. Tiles are also DnD drag sources (drag onto canvas to insert at position). No external icon library — text/unicode glyphs only, consistent with existing monospace aesthetic.

### `web/src/lib/components/builder/FieldCanvas.svelte`

Centre panel. Renders the ordered list of `BuilderField` items using `svelte-dnd-action`. Drop zone between fields allows reordering via drag-and-drop.

Each field row in the canvas:
- Drag handle on the left
- `FieldRow.svelte` inline component: shows field type badge + label from active locale translation
- "Required" toggle badge
- Delete icon (×)
- Click anywhere on row → `selectedFieldId = field.id`
- Highlighted when selected (border or background accent)

Fields with missing translation in the active locale show a ⚠ amber indicator.

`section_break` fields render as a horizontal rule with an optional label preview.

### `web/src/lib/components/builder/PropertiesPanel.svelte`

Right panel. Conditionally renders based on `selectedField`:

- **No selection:** Prompt: "Select a field to edit its properties."
- **Field selected:** Two tabs — "Settings" and "Translation".

**Settings tab** — field-type-specific controls:
- `short_text` / `long_text`: maxLength input, minRows (long_text only)
- `multiple_choice` / `checkboxes` / `dropdown`: option list (add/remove/reorder options), allowOther toggle (multiple_choice), minSelect/maxSelect (checkboxes), searchable toggle (dropdown)
- `date_time`: mode selector (date/time/datetime), min/max date inputs
- `rating`: scale selector (5 or 10), shape selector (star/number)
- `section_break`: no config options (only translation tab is relevant)
- All fields: "Required" toggle

**Translation tab** — shows `TranslationEditor.svelte` for the selected field.

When no field is selected, a second state shows form-level settings:
- Form description textarea (bound to `activeTranslation.formDescription`)
- `convoCompletionMessage` (shown only when `layout === 'convo'`)
- `convoAllowEdit` toggle (shown only when `layout === 'convo'`)

### `web/src/lib/components/builder/TranslationEditor.svelte`

Shows the translation slots for the selected field (or form-level slots if `fieldId` is null). Displays:
- `label` textarea (required — amber highlight if empty in default locale)
- `helpText` textarea (optional)
- `placeholder` input (optional, for text fields)
- `options` inputs (for choice fields — parallel to `config.options`, one input per option)

When `activeLocale !== defaultLocale`, each slot shows the default-locale value in grey below the input as a reference. Missing translations are amber-highlighted.

---

## Step 9 — forms.ts additions

Add to `web/src/lib/forms.ts`:

```typescript
/**
 * Publish (or re-publish) a form: generates a fresh renderKey, re-encrypts
 * the schema for respondents, and PUTs both encrypted schemas + publicFormKey.
 * Returns the share URL fragment (everything after the host).
 */
export async function publishForm(
  masterKey: CryptoKey,
  formId: string,
  schema: BuilderSchema
): Promise<{ shareUrl: string; renderKey: CryptoKey }>

/**
 * Fetch a specific versioned schema snapshot and decrypt it.
 * Used by the response viewer (Phase 6).
 */
export async function getSchemaVersion(
  masterKey: CryptoKey,
  formId: string,
  version: number
): Promise<BuilderSchema>
```

`publishForm` is the extracted publish logic used by both the initial publish and renderKey rotation. The caller is responsible for composing the full share URL from the returned fragment.

---

## Step 10 — Forms list page update

Update `web/src/routes/(app)/forms/+page.svelte`:

- "New form" button: navigate to `/forms/new` (replace `alert()`).
- Add "Edit" button per row: navigates to `/forms/{id}/edit`.
- Add form title column: decrypt-free, using form ID. Title is only readable after opening the builder.

---

## Step 11 — Install dependency

```bash
# in web/
pnpm add svelte-dnd-action
```

No other new dependencies. No UI component library. Icons are unicode glyphs.

---

## File checklist

**New files:**
- `migrations/0007_schema_versions.up.sql`
- `migrations/0007_schema_versions.down.sql`
- `web/src/lib/types/builder.ts`
- `web/src/lib/stores/builder.svelte.ts`
- `web/src/routes/(app)/forms/new/+page.svelte`
- `web/src/routes/(app)/forms/new/+page.ts`
- `web/src/routes/(app)/forms/[id]/edit/+page.svelte`
- `web/src/routes/(app)/forms/[id]/edit/+page.ts`
- `web/src/lib/components/builder/FieldPalette.svelte`
- `web/src/lib/components/builder/FieldCanvas.svelte`
- `web/src/lib/components/builder/PropertiesPanel.svelte`
- `web/src/lib/components/builder/TranslationEditor.svelte`

**Modified files:**
- `internal/db/queries/forms.sql` — add InsertSchemaVersion, GetSchemaVersion, ListSchemaVersions
- `internal/forms/service.go` — snapshot insert on CreateForm + UpdateFormSchema; add GetSchemaVersion method
- `internal/forms/handler.go` — add GET /api/forms/{id}/schema-versions/{version}
- `internal/server/server.go` — mount new handler
- `web/src/lib/types/crypto.ts` — re-export BuilderSchema as FormSchema
- `web/src/lib/forms.ts` — add publishForm, getSchemaVersion
- `web/src/routes/(app)/forms/+page.svelte` — wire New Form nav + Edit button

**Run after SQL changes:**
```bash
sqlc generate
```

**Run for new frontend dependency:**
```bash
cd web && pnpm add svelte-dnd-action
```

---

## Testing exit criterion

All of the following must be verifiable end-to-end:

1. **Create + build:** Click "New Form" → builder opens with empty canvas. Add 3+ field types including one choice field. Verify canvas shows them ordered correctly.

2. **Translation:** Switch locale to "fr". Verify default-locale values shown as reference. Fill in French translations. Verify no amber warnings remain.

3. **Drag reorder:** Drag a field to a new position. Verify order persists after auto-save (reload builder, confirm order).

4. **Auto-save:** Edit a field label. Wait 2 seconds. Verify "Saved" indicator. Check DB: `SELECT schema_version FROM forms` incremented. Check `form_schema_versions` has a new row.

5. **Schema snapshots:** After 3 edits, `SELECT version FROM form_schema_versions WHERE form_id = X ORDER BY version` returns 3 rows (versions 2, 3, 4 — version 1 from creation).

6. **Preview mode:** Toggle preview. Verify field palette and properties panel are hidden. Verify form renders with field labels from active locale.

7. **Publish:** Click Publish. Verify share URL modal appears with `#rk=` fragment. Verify `render_encrypted_schema` is updated in DB. Verify old `renderKey` (if rotated) decrypts to different bytes.

8. **Share URL:** Open share URL in a private window (no auth). Verify form title is visible (Phase 6 will render fields; Phase 5 just confirms schema decrypts correctly in `f/[id]/+page.svelte`).

9. **Schema confidentiality:** `SELECT encode(encrypted_schema, 'hex') FROM forms LIMIT 1` — opaque. No field labels, titles, or option text visible in plaintext in the DB.
