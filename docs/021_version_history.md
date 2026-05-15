# Version History, Rollbacks, and Undo/Redo

## Overview

Two independent features:

1. **Undo/Redo** — in-session history stack in `builder.svelte.ts`, no backend changes
2. **Version History + Rollbacks** — published-version snapshots in the DB, new backend endpoints, new UI panel

---

## Feature 1: Undo / Redo (Frontend Only)

### `web/src/lib/stores/builder.svelte.ts`

Add two new `$state` arrays: `undoStack: BuilderSchema[]` and `redoStack: BuilderSchema[]`, capped at 50 entries.

Add a `pushHistory()` helper that every schema-mutating action calls *before* modifying `schema`:

```
pushHistory() → undoStack.push(snapshot of current schema), clear redoStack
```

Wrap these actions with `pushHistory()`:
`addField`, `addFieldAt`, `removeField`, `duplicateField`, `reorderFields`, `updateField`, `updateFieldConfig`, `updateTranslation`, `addLocale`, `removeLocale`, `setLayout`, `setConvoAllowEdit`, `setShowWatermark`

Add `undo()` and `redo()` methods:
- `undo()`: pop undoStack → push current schema to redoStack → set schema → markDirty
- `redo()`: pop redoStack → push current schema to undoStack → set schema → markDirty

Add read-only getters `canUndo` and `canRedo` (booleans) to the `BuilderStore` interface.

Add a `restoreVersion(restoredSchema: BuilderSchema)` action:
- pushHistory() first (so undo works after a restore)
- set schema = restoredSchema
- markDirty (triggers auto-save)

### `web/src/routes/(app)/forms/[id]/edit/+page.svelte`

- Add `<svelte:window onkeydown={handleKeydown} />` handler:
  - `Ctrl/Cmd+Z` → `store.undo()`
  - `Ctrl/Cmd+Shift+Z` or `Ctrl+Y` → `store.redo()`
- Add undo/redo icon buttons (`Undo2` / `Redo2` from Lucide) in the toolbar, disabled when `!store.canUndo` / `!store.canRedo`

---

## Feature 2: Version History + Rollbacks (Full Stack)

### 2a. Database migration — `migrations/0026_version_history.up/down.sql`

```sql
-- up
ALTER TABLE form_schema_versions ADD COLUMN is_publish BOOLEAN NOT NULL DEFAULT FALSE;
```

```sql
-- down
ALTER TABLE form_schema_versions DROP COLUMN is_publish;
```

Purpose: distinguish auto-save versions from publish snapshots. Only publish snapshots are shown in the history panel.

### 2b. SQL queries — `internal/db/queries/forms.sql`

Add two queries:

```sql
-- name: ListPublishedVersions :many
SELECT version, created_at FROM form_schema_versions
WHERE form_id = $1 AND is_publish = TRUE ORDER BY version DESC;

-- name: RestoreSchemaVersion :one
UPDATE forms
SET encrypted_schema        = (SELECT encrypted_schema FROM form_schema_versions WHERE form_id = $1 AND version = $2),
    schema_version          = schema_version + 1,
    has_unpublished_changes = TRUE,
    updated_at              = NOW()
WHERE id = $1 AND workspace_id = $3
RETURNING schema_version;
```

Run `sqlc generate` to regenerate `internal/db/queries/*.go`.

### 2c. Backend service — `internal/forms/service.go`

**DB interface** — add two new methods:

```go
ListPublishedVersions(ctx context.Context, formID string) ([]queries.ListPublishedVersionsRow, error)
RestoreSchemaVersion(ctx context.Context, arg queries.RestoreSchemaVersionParams) (int32, error)
```

**`PublishForm` method** — wrap in a transaction; after updating the form row, insert a version snapshot with `is_publish = true`:

```go
qtx.InsertSchemaVersion(ctx, queries.InsertSchemaVersionParams{
    FormID:          formID,
    Version:         newVersion,
    EncryptedSchema: currentEncryptedSchema,
    IsPublish:       true,
})
```

This requires fetching the current `encrypted_schema` before publishing via `GetFormByWorkspace`.

**New `ListPublishedVersions(ctx, workspaceID, formID string)` method** — validates workspace membership, then calls DB.

**New `RestoreSchemaVersion(ctx, workspaceID, formID string, version int32)` method** — validates workspace membership, calls DB query, returns new version number.

### 2d. Backend handler — `internal/forms/handler.go`

Add two new handlers and register routes:

```
GET  /api/forms/{id}/schema-versions                   → listSchemaVersions
POST /api/forms/{id}/schema-versions/{version}/restore → restoreSchemaVersion
```

`listSchemaVersions` response:

```json
{ "versions": [{ "version": 5, "createdAt": "2026-05-01T12:00:00Z" }] }
```

`restoreSchemaVersion` response:

```json
{ "schemaVersion": 12 }
```

After receiving the restore response, the client calls `store.load()` to pull the newly restored draft.

### 2e. Frontend API — `web/src/lib/forms.ts`

Add three functions:

```ts
// List published version snapshots for a form
listFormVersions(formId: string): Promise<{ version: number; createdAt: string }[]>

// Fetch and decrypt a historical version
getFormVersion(
  masterKey: CryptoKey,
  formId: string,
  version: number,
  formKey?: CryptoKey
): Promise<BuilderSchema>

// Server-side restore — copies historical blob back to the draft
restoreFormVersion(formId: string, version: number): Promise<{ schemaVersion: number }>
```

`getFormVersion` hits the existing `GET /api/forms/{id}/schema-versions/{version}` endpoint and decrypts the blob using the same `decryptSchemaCompat` helper used in `getForm`.

### 2f. New component — `web/src/lib/components/builder/VersionHistoryPanel.svelte`

A right slide-in panel, positioned identically to `FormSettingsPanel`, visible when `store.showVersionHistory === true`.

Content:
- Header: "Version history" + close button
- Loads versions on mount via `listFormVersions(formId)`
- Each row shows version number, formatted date, and two actions:
  - **Preview** — calls `getFormVersion(...)`, shows schema in a read-only `FormPreview` overlay
  - **Restore** — inline confirmation ("Restore this version? Current draft will be overwritten.") → on confirm, calls `restoreFormVersion(formId, version)` then `store.load()`
- Empty state when no published versions exist: "Publish your form to create your first version snapshot"

### 2g. Editor page — `web/src/routes/(app)/forms/[id]/edit/+page.svelte`

- Import and render `<VersionHistoryPanel {store} {formId} />` alongside `FormSettingsPanel`
- Add a "History" toolbar button (`History` icon from Lucide) that toggles `store.showVersionHistory`
- Add `showVersionHistory` and `setShowVersionHistory` to `builder.svelte.ts` store state and interface

---

## File Checklist

| # | File | Change |
|---|------|--------|
| 1 | `migrations/0026_version_history.up.sql` | New — add `is_publish` column |
| 2 | `migrations/0026_version_history.down.sql` | New — drop column |
| 3 | `internal/db/queries/forms.sql` | Add `ListPublishedVersions`, `RestoreSchemaVersion` queries |
| 4 | `internal/db/queries/*.go` | Regenerated by sqlc |
| 5 | `internal/forms/service.go` | Add `ListPublishedVersions`, `RestoreSchemaVersion`; update `PublishForm` to snapshot with `is_publish=true` |
| 6 | `internal/forms/handler.go` | Add `listSchemaVersions`, `restoreSchemaVersion` handlers + routes |
| 7 | `web/src/lib/stores/builder.svelte.ts` | Undo/redo stack, `restoreVersion`, `showVersionHistory` state |
| 8 | `web/src/lib/forms.ts` | `listFormVersions`, `getFormVersion`, `restoreFormVersion` |
| 9 | `web/src/lib/components/builder/VersionHistoryPanel.svelte` | New component |
| 10 | `web/src/routes/(app)/forms/[id]/edit/+page.svelte` | Keyboard shortcuts, toolbar buttons, panel wiring |

---

## Key Design Decisions

- **Version scope**: only published snapshots appear in the history panel, not every auto-save. This keeps the list meaningful and short.
- **Rollback is server-side**: the encrypted blob is copied directly on the server — no client-side re-encryption required, since the form key hasn't changed.
- **After restore, `store.load()` is called**: re-decrypts the restored schema fresh from the server, avoiding stale state.
- **Undo after restore**: `restoreVersion()` pushes the current draft to undoStack first, so users can undo a restore via `Ctrl+Z`.
- **Undo/redo is session-scoped**: history clears on page reload — standard editor behavior.
