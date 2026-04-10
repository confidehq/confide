# Phase 6 — Form Runtime + Response Viewer

**Status:** Planning
**Exit criterion:** A respondent can open a share link, fill in a form, and submit. A creator can open the responses page, decrypt any response, and read the answers — all without the server ever seeing plaintext.

---

## Scope

**Part A — Form Runtime:** Replace the Phase 5 placeholder in `/f/[id]` with a working form renderer. Supports `scroll` and `steps` layouts. `convo` layout is deferred.

**Part B — Response Viewer:** New creator-facing route at `/forms/[id]/responses`. Lists responses with metadata; decrypts on demand; renders answers against the exact schema version each response was submitted under.

**Deferred to Phase 7:** Export (CSV/JSON), analytics, archive/soft-delete, bulk operations, `convo` layout.

---

## DQ6 — Answer format contract

Each field type stores its answer as a single, well-defined TypeScript type:

| Field type | Answer type | Notes |
|---|---|---|
| `short_text` | `string` | Empty string for blank |
| `long_text` | `string` | Empty string for blank |
| `multiple_choice` | `string` | Option ID; or `"other:<freetext>"` if allowOther |
| `checkboxes` | `string[]` | Array of option IDs |
| `dropdown` | `string` | Option ID |
| `date_time` | `string` | ISO 8601 (date-only, time-only, or full datetime) |
| `rating` | `number` | Integer 1–scale |
| `section_break` | `null` | Always null — section breaks have no answer |

This is the `answers` map in the response payload. Unanswered optional fields are omitted (not stored as null). Required fields that are empty produce a validation error before submission.

---

## DQ6b — Schema version lookup for response viewer

Every response row carries `schema_version`. The response viewer must render answers against the schema **as it was when the respondent submitted**, not the current published version.

Fetch via `GET /api/forms/{id}/schema-versions/{version}` (authenticated, owner-only — already implemented in Phase 5). Decrypt with `formKey` (derived from `masterKey`). Cache per `formId:version` key for the lifetime of the page — do not re-fetch on every expand.

---

## Step 1 — Field input components

All in `web/src/lib/components/form/`. Each component receives:
- `field: BuilderField` — config, type, required flag
- `translation: FieldTranslation` — label, helpText, placeholder, options (from active locale)
- `value: AnswerValue` — current answer (undefined = untouched)
- `error: string | null` — validation message to show
- `onchange: (v: AnswerValue) => void` — call on any user change

Components are stateless — they receive current value and fire change events. The form renderer owns all answer state.

### `ShortTextInput.svelte`

```svelte
<label>{translation.label}</label>
{#if translation.helpText}<p class="help">{translation.helpText}</p>{/if}
<input type="text"
  bind:value
  maxlength={field.config.maxLength}
  placeholder={translation.placeholder ?? ''}
  on:input={e => onchange(e.currentTarget.value)}
/>
{#if error}<p class="error">{error}</p>{/if}
```

### `LongTextInput.svelte`

Same as ShortTextInput but `<textarea>`. Respects `minRows` (sets `rows` attribute). Auto-grows via CSS `field-sizing: content` (progressive enhancement — falls back to fixed rows).

### `MultipleChoiceInput.svelte`

Radio group. `field.config.options` array defines the choices; `translation.options[i]` provides the label for option at index `i` (parallel arrays by order). If `config.allowOther` is true, add a final "Other:" radio + text input. When "Other" is selected, answer becomes `"other:<freetext>"`.

### `CheckboxesInput.svelte`

Checkbox group. Same option label resolution as MultipleChoiceInput. Answer is `string[]` of checked option IDs. Enforce `minSelect` / `maxSelect` from config — show error if violated on blur/submit (not on every keystroke).

### `DropdownInput.svelte`

`<select>` element. If `config.searchable` is true, replace with a filter-input + option list pattern (no external library — native `<datalist>` or a simple custom dropdown). Default option: `"— Select —"` (not selectable).

### `DateTimeInput.svelte`

Single `<input>` with `type="date"`, `type="time"`, or `type="datetime-local"` based on `config.mode`. Apply `min` and `max` from config as HTML attributes. Normalize value to ISO 8601 on change.

### `RatingInput.svelte`

Renders `config.scale` (5 or 10) interactive items. If `config.shape === 'star'`, use `★` / `☆` toggle. If `config.shape === 'number'`, render numbered buttons `1`–`N`. Answer is the selected integer. Highlight all items up to the current rating (fill left-to-right). Clicking the active rating again clears it (sets to 0, i.e., unanswered).

### `SectionBreakInput.svelte`

Display-only. Renders `translation.label` as a section heading (`<h2>`) and `translation.helpText` as a paragraph beneath it. Returns no answer — always `null`.

---

## Step 2 — Form state (`formState.svelte.ts`)

`web/src/lib/stores/formState.svelte.ts`

Runes-based store holding respondent-side form state for a single form session. Created once per page load; not persisted.

```typescript
// State
let schema = $state<BuilderSchema | null>(null);
let locale = $state<string>('en');
let answers = $state<Record<string, AnswerValue>>({});
let errors = $state<Record<string, string>>({});
let submitting = $state(false);
let submitted = $state(false);
let submitError = $state<string | null>(null);
let currentStep = $state(0);  // for 'steps' layout

// Derived
const translation = $derived(
  schema
    ? (schema.translations[locale] ?? schema.translations[schema.defaultLocale])
    : null
);
const visibleFields = $derived(
  schema?.fields.filter(f => f.type !== 'section_break' || schema.layout !== 'convo') ?? []
);
const stepsGroups = $derived(groupFieldsByStep(schema));

// Actions
function setAnswer(fieldId: string, value: AnswerValue): void
function validate(): boolean   // sets errors, returns true if no errors
function validateField(fieldId: string): boolean
function nextStep(): void      // for 'steps' layout
function prevStep(): void
```

`groupFieldsByStep(schema)` splits `fields` at each `section_break`: each section_break starts a new step, and the fields between breaks form the step's content. The very first step contains all fields before the first section_break.

---

## Step 3 — Form renderers

### `ScrollRenderer.svelte`

`web/src/lib/components/form/ScrollRenderer.svelte`

Renders all fields in one scrollable page. No step navigation.

Layout:
```
[Form title]
[Form description]
[field 1 input]
[field 2 input]
...
[Submit button]
[Error message if submit failed]
```

On submit:
1. Call `validate()` — if errors exist, scroll to first error field (`document.getElementById(fieldId).scrollIntoView()`).
2. Build payload: `{ submittedAt: new Date().toISOString(), locale, answers }`.
3. Call `submitResponse(formId, publicFormKey, payload)` (already in `forms.ts`).
4. On success: `submitted = true` → show completion message.
5. On error: `submitError = message` → show inline error, re-enable submit button.

### `StepsRenderer.svelte`

`web/src/lib/components/form/StepsRenderer.svelte`

Splits fields into steps using `section_break` fields as dividers. Each step is one "page."

Layout:
```
[Step N of M indicator]
[Step title (section_break label, or form title for step 1)]
[Field inputs for this step]
[Back | Next / Submit buttons]
```

`Next`: validates current step's fields only → advance `currentStep`. `Back`: go to previous step (no re-validation). Final step's Next becomes Submit, which validates all fields and calls `submitResponse`.

Progress indicator: `Step N of M` text (no progress bar — keep it minimal).

---

## Step 4 — Update `/f/[id]/+page.svelte`

Replace the placeholder with the real renderer. The page already:
- Parses `#rk=` from the URL fragment
- Calls `getPublicSchema(formId, renderKey)` and decrypts the schema
- Has `'loading' | 'ready' | 'closed' | 'invalid' | 'error'` state machine

Changes:
1. Parse `?locale=<locale>` from the query string; fall back to `schema.defaultLocale`.
2. Initialize `formState` store with `schema` and `locale`.
3. In the `ready` branch, replace the placeholder `<p>` with:

```svelte
{#if $submitted}
  <div class="shell">
    <p>{$translation?.convoCompletionMessage ?? 'Your response has been submitted.'}</p>
  </div>
{:else if schema.layout === 'steps'}
  <StepsRenderer {schema} {formId} {publicFormKey} />
{:else}
  <ScrollRenderer {schema} {formId} {publicFormKey} />
{/if}
```

Pass `formId` and the decoded `publicFormKey` (32-byte `Uint8Array`) as props to the renderer. The renderers own the `formState` instance.

The page stays outside the `(app)` layout group — it has no auth requirement and must remain publicly accessible.

---

## Step 5 — `forms.ts` additions

Add two functions not yet present:

```typescript
/**
 * Fetch a single response record (encrypted) by ID.
 */
export async function getResponse(
  formId: string,
  responseId: string
): Promise<ResponseRecord>

/**
 * Delete a single response.
 */
export async function deleteResponse(
  formId: string,
  responseId: string
): Promise<void>
```

`ResponseRecord` (already defined from Phase 4):
```typescript
interface ResponseRecord {
  id: string;
  formId: string;
  receivedAt: string;
  schemaVersion: number;
  encryptedData: string;       // base64
  ephemeralPublicKey: string;  // base64
}
```

`decryptResponseRecord(masterKey, formId, record)` is already implemented in Phase 4 — no changes needed.

---

## Step 6 — Response list route

### `web/src/routes/(app)/forms/[id]/responses/+page.ts`

```typescript
export const ssr = false;
export const prerender = false;
```

### `web/src/routes/(app)/forms/[id]/responses/+page.svelte`

Creator-facing page listing all responses for a form. Requires auth (masterKey from auth store — re-auth modal already handles expiry).

**State:**
```typescript
let responses = $state<ResponseRecord[]>([]);
let cursor = $state<string | null>(null);
let hasMore = $state(false);
let loading = $state(true);
let formMeta = $state<FormMeta | null>(null);   // from getForm()
let schemaCache = $state<Map<number, BuilderSchema>>(new Map());
let decrypted = $state<Map<string, DecryptedResponse>>(new Map());
let decrypting = $state<Set<string>>(new Set());
```

`DecryptedResponse`:
```typescript
interface DecryptedResponse {
  submittedAt: string;
  locale: string;
  answers: Record<string, AnswerValue>;
  schema: BuilderSchema;    // the exact version this response was submitted under
}
```

**On mount:**
1. `getForm(masterKey, formId)` — decrypt form metadata (confirm ownership).
2. `listResponses(formId, { limit: 25 })` — load first page.
3. Render list.

**List layout (table or rows):**
```
ID (truncated)  |  Received         |  Schema v  |  [Decrypt]  [Delete]
rsp_abc…        |  Mar 20 14:31     |  v3        |  [Decrypt]  [×]
rsp_def…        |  Mar 20 09:12     |  v3        |  [Decrypt]  [×]
```

**"Load more" button** appears if `hasMore`. Appends next page to `responses` array using `cursor`.

**Decrypt flow (per response):**
1. Mark response ID in `decrypting` set → show spinner on button.
2. Look up `schemaCache.get(record.schemaVersion)`. If miss: call `getSchemaVersion(masterKey, formId, version)` → insert into cache.
3. Call `decryptResponseRecord(masterKey, formId, record)` → get `{ submittedAt, locale, answers }`.
4. Store in `decrypted` map.
5. Remove from `decrypting` set → expand inline to show decrypted content.

**Decrypted response display (inline expand):**
```
Submitted: 2026-03-20 14:31 UTC  ·  Locale: en

[field label]
  [answer rendered as text]

[field label]
  [answer rendered as text]
```

Use `renderAnswer(field, translation, answer)` — a pure helper function:
- `short_text` / `long_text`: display as plain text
- `multiple_choice` / `dropdown`: look up option label from `translation.options[optionIndex]` (match by option ID to `config.options[i].id`)
- `checkboxes`: comma-separated option labels
- `date_time`: format with `Intl.DateTimeFormat` based on mode
- `rating`: `N / scale` (e.g. `4 / 5`)
- `section_break`: skip entirely

**Delete flow:**
1. Show inline confirmation: "Delete this response? This cannot be undone." with [Confirm] [Cancel].
2. On confirm: call `deleteResponse(formId, responseId)`.
3. Remove from `responses` array and `decrypted` map.
4. Update local response count (decrement).

**Page header:**
- Form title (form ID if title not yet decrypted)
- Response count from `formMeta.responseCount`
- Back link: `← Back to forms`

---

## Step 7 — Link responses from the forms list + builder

### `web/src/routes/(app)/forms/+page.svelte`

Add a "Responses" link per row in the forms table:
```
[Edit]  [Responses (N)]  [Copy link]  [Delete]
```
`N` is the `responseCount` from the form list API response (unencrypted metadata — already returned).

### `web/src/routes/(app)/forms/[id]/edit/+page.svelte`

Add a "Responses" link in the toolbar area (top right, alongside Publish):
```
[Responses (N)]  [Publish]
```

---

## Step 8 — Validation logic

Centralize in `web/src/lib/validation.ts` (new file, pure functions):

```typescript
export function validateAnswer(
  field: BuilderField,
  value: AnswerValue
): string | null {
  if (field.required && isEmpty(value)) return 'This field is required.';
  if (field.type === 'checkboxes') {
    const cfg = field.config as CheckboxesConfig;
    const arr = (value as string[]) ?? [];
    if (cfg.minSelect && arr.length < cfg.minSelect)
      return `Select at least ${cfg.minSelect} options.`;
    if (cfg.maxSelect && arr.length > cfg.maxSelect)
      return `Select at most ${cfg.maxSelect} options.`;
  }
  if (field.type === 'date_time' && value) {
    const cfg = field.config as DateTimeConfig;
    if (cfg.min && String(value) < cfg.min) return `Date must be on or after ${cfg.min}.`;
    if (cfg.max && String(value) > cfg.max) return `Date must be on or before ${cfg.max}.`;
  }
  return null;
}

function isEmpty(value: AnswerValue): boolean {
  if (value === null || value === undefined) return true;
  if (typeof value === 'string') return value.trim() === '';
  if (Array.isArray(value)) return value.length === 0;
  return false;
}
```

`validateAll(schema, answers)` calls `validateAnswer` for every field and returns a `Record<fieldId, string>` errors map.

---

## File checklist

**New files:**
- `web/src/lib/components/form/ShortTextInput.svelte`
- `web/src/lib/components/form/LongTextInput.svelte`
- `web/src/lib/components/form/MultipleChoiceInput.svelte`
- `web/src/lib/components/form/CheckboxesInput.svelte`
- `web/src/lib/components/form/DropdownInput.svelte`
- `web/src/lib/components/form/DateTimeInput.svelte`
- `web/src/lib/components/form/RatingInput.svelte`
- `web/src/lib/components/form/SectionBreakInput.svelte`
- `web/src/lib/components/form/ScrollRenderer.svelte`
- `web/src/lib/components/form/StepsRenderer.svelte`
- `web/src/lib/stores/formState.svelte.ts`
- `web/src/lib/validation.ts`
- `web/src/routes/(app)/forms/[id]/responses/+page.ts`
- `web/src/routes/(app)/forms/[id]/responses/+page.svelte`

**Modified files:**
- `web/src/routes/f/[id]/+page.svelte` — replace placeholder with ScrollRenderer/StepsRenderer dispatch
- `web/src/lib/forms.ts` — add `getResponse()`, `deleteResponse()`
- `web/src/routes/(app)/forms/+page.svelte` — add Responses link per row
- `web/src/routes/(app)/forms/[id]/edit/+page.svelte` — add Responses link in toolbar

**No backend changes required.** All necessary endpoints were implemented in Phases 4–5:
- `GET /api/f/{id}/schema` — public schema fetch ✓
- `POST /relay/submit` — response submission ✓
- `GET /api/forms/{id}/responses` — paginated list ✓
- `GET /api/forms/{id}/responses/{rid}` — single response ✓
- `DELETE /api/forms/{id}/responses/{rid}` — delete ✓
- `GET /api/forms/{id}/schema-versions/{version}` — historical schema ✓

---

## Testing exit criterion

All of the following must be verifiable end-to-end in a single browser session:

1. **Scroll form:** Open a share URL with `#rk=` fragment. Verify form title and all fields render with correct labels. Fill in all required fields. Click Submit. Verify "Your response has been submitted." appears.

2. **Validation:** Leave a required field empty and click Submit. Verify field shows an error message and submit is blocked. Fill it in. Verify error clears. Submit succeeds.

3. **Steps form:** Create a form with at least one `section_break` field. Publish and open. Verify fields are split across steps. Next button advances step (and validates current step). Back returns to previous step without validation. Final step submits.

4. **Closed form:** Set form status to `closed`. Open the share URL. Verify "This form is no longer accepting responses." message.

5. **Localization:** Create a form with `en` and `fr` locales, different labels per locale. Open share URL with `?locale=fr`. Verify French labels appear. Open without param. Verify default locale labels appear.

6. **Multiple choice with "other":** Submit a multiple_choice field with the "Other" option + freetext. Open the response viewer. Decrypt the response. Verify the answer displays the freetext correctly.

7. **Response viewer:** Open `/forms/{id}/responses`. Verify response list loads with received dates and schema version numbers.

8. **Decrypt response:** Click Decrypt on a response. Verify answers appear inline against correct field labels. Verify the schema version shown matches the response's `schema_version`.

9. **Schema version mismatch:** Edit and re-publish the form (bumps schema version). Submit a new response. In the response viewer, decrypt both the old and new response. Verify each shows field labels from the version it was submitted under, not the current version.

10. **Delete response:** Click delete on a response, confirm. Verify it disappears from the list. Verify `response_count` on the form row decrements.

11. **E2E confidentiality:** `SELECT encode(encrypted_data, 'hex'), encode(ephemeral_public_key, 'hex') FROM responses LIMIT 1` — verify both columns are opaque bytes. No answer text visible in the database.
