# 019 — Privacy-Preserving Analytics

## Overview

Two-tier analytics system gated by workspace plan (`free` / `pro` / `org`). All metrics are
aggregated at write time — no raw event log, no PII ever stored.

### Design Principles

- **No PII stored** — IP is used only at the edge to derive a country code, then discarded.
  No user IDs, session tokens, fingerprints, or cookies.
- **Aggregation-only storage** — events upsert directly into pre-aggregated counters
  (`count + 1`). No replay pipeline or rollup job required.
- **Turnstile model** — the form frontend emits checkpoint events at each stage
  (`view → start → step_N → submit`). Drop-off rates are derived at query time, not stored.
- **Plan gating at the query layer** — one beacon endpoint serves all plans; the read API
  enforces what each plan can see.

---

## Tier Comparison

| Metric                                     | Free | Pro / Org |
|--------------------------------------------|:----:|:---------:|
| History window                             | 30 d | 365 d     |
| Total views + submissions                  | ✓    | ✓         |
| Daily trend chart                          | ✓    | ✓         |
| Submission rate                            | ✓    | ✓         |
| Turnstile / step-level funnel              | —    | ✓         |
| Device breakdown (mobile / desktop / tablet) | —  | ✓         |
| Country breakdown                          | —    | ✓         |
| Referrer domain breakdown                  | —    | ✓         |

---

## Phase 1 — Database

**Migration `0026_analytics`**

```sql
CREATE TABLE form_analytics (
    form_id    TEXT      NOT NULL REFERENCES forms(id) ON DELETE CASCADE,
    period     DATE      NOT NULL,
    event_type TEXT      NOT NULL
                           CHECK (event_type IN ('view','start','step','submit')),
    step       SMALLINT  NOT NULL DEFAULT 0,
    device     TEXT      NOT NULL DEFAULT 'unknown'
                           CHECK (device IN ('mobile','desktop','tablet','unknown')),
    country    TEXT      NOT NULL DEFAULT '',
    referrer   TEXT      NOT NULL DEFAULT '',
    count      BIGINT    NOT NULL DEFAULT 1,
    PRIMARY KEY (form_id, period, event_type, step, device, country, referrer)
);

CREATE INDEX idx_form_analytics_lookup
    ON form_analytics (form_id, period DESC);
```

- **Composite PK** makes every `INSERT … ON CONFLICT DO UPDATE SET count = count + 1`
  atomic and idempotent.
- `period` is truncated to day (UTC) — finest resolution offered to any plan, reducing
  re-identification risk.
- Cascade delete keeps the table clean when forms are removed.

### Column semantics

| Column | Description |
|--------|-------------|
| `event_type` | `view` page load · `start` first field interaction · `step` turnstile checkpoint · `submit` completed submission |
| `step` | Field index (0-based) for `step` events; `0` for all others |
| `device` | Coarse UA classification: `mobile`, `desktop`, `tablet`, `unknown` |
| `country` | ISO-3166-1 alpha-2, derived from request header; `''` if unavailable |
| `referrer` | Domain only (e.g. `twitter.com`); `''` if none |

---

## Phase 2 — Backend Package `internal/analytics`

### `service.go`

- `RecordEvent(ctx, formID, event, step, device, country, referrer string)` — executes the upsert.
- `QueryBasic(ctx, workspaceID, formID string, days int) (BasicResult, error)` — views,
  submissions, and daily trend. Validates the form belongs to the workspace.
- `QueryAdvanced(ctx, workspaceID, formID string, days int) (AdvancedResult, error)` — extends
  basic with device, country, referrer, and step funnel breakdowns.

Plan enforcement lives in the handler (not the service) so the service remains independently
testable.

### `handler.go`

| Route | Auth | Description |
|-------|------|-------------|
| `POST /relay/analytics` | None | Public beacon — always returns 202, never leaks errors |
| `GET /api/workspaces/{wid}/forms/{fid}/analytics` | Session | Returns basic or advanced based on `workspace.plan` |

### Beacon request body

```json
{ "formId": "abc", "event": "view", "step": 0, "referrer": "twitter.com" }
```

- `referrer` is domain only — frontend strips path/query before sending.
- `step` is meaningful only for `event: "step"`.
- `device` is derived server-side from `User-Agent` (coarse classification).
- `country` is derived from `CF-IPCountry` (Cloudflare) or `X-GeoIP-Country` (Traefik).
  Falls back to `""`. The IP is never logged or stored.

### Query response — free

```json
{
  "plan": "free",
  "range": 30,
  "total": { "views": 1200, "submissions": 480, "rate": 0.40 },
  "daily": [
    { "date": "2026-05-01", "views": 42, "submissions": 17 }
  ]
}
```

### Query response — pro (superset of free)

```json
{
  "plan": "pro",
  "range": 90,
  "total": { "views": 1200, "submissions": 480, "rate": 0.40 },
  "daily": [ ... ],
  "devices":   { "mobile": 700, "desktop": 460, "tablet": 40 },
  "countries": [{ "code": "US", "count": 800 }, { "code": "DE", "count": 200 }],
  "referrers": [{ "domain": "twitter.com", "count": 300 }],
  "funnel": [
    { "stage": "view",   "count": 1200 },
    { "stage": "start",  "count": 900  },
    { "stage": "step_1", "count": 750  },
    { "stage": "step_2", "count": 620  },
    { "stage": "submit", "count": 480  }
  ]
}
```

---

## Phase 3 — sqlc Queries

Four queries added to `internal/db/queries/analytics.sql`:

1. **`UpsertAnalyticsEvent`** — atomic increment via `ON CONFLICT DO UPDATE`
2. **`GetAnalyticsBasic`** — daily aggregation of `view` + `submit` within date range
3. **`GetAnalyticsAdvanced`** — extends basic with device / country / referrer / step breakdowns
4. **`GetAnalyticsFunnel`** — step-ordered turnstile counts for the funnel chart

---

## Phase 4 — Frontend Beacon

Small module embedded in the public form renderer (no third-party dependencies):

```typescript
function beacon(event: 'view' | 'start' | 'step' | 'submit', step = 0) {
  const referrer = document.referrer ? new URL(document.referrer).hostname : ''
  navigator.sendBeacon('/relay/analytics', JSON.stringify({
    formId: FORM_ID, event, step, referrer,
  }))
}
```

- Uses `navigator.sendBeacon` — fire-and-forget, survives page unload (critical for `submit`).
- No cookies, no local storage, no cross-origin requests.
- Referrer domain computed client-side from `document.referrer`.

Beacon calls:

| Trigger | Call |
|---------|------|
| Page load | `beacon('view')` |
| First field focus | `beacon('start')` |
| Required field completed | `beacon('step', fieldIndex)` |
| Form submitted | `beacon('submit')` |

---

## Phase 5 — Analytics Dashboard UI

Both tiers are surfaced in the form detail view inside the workspace dashboard.

### Free card

- Three KPI tiles: **Views this month**, **Submissions**, **Rate**.
- Area chart: 30-day daily trend (pure SVG path from API data).
- Upsell CTA: "Unlock device breakdown and step-by-step funnel with Pro."

### Pro panel (extends free card)

- **Funnel visualization** — horizontal bars with drop-off % annotations between stages.
- **Device donut chart**.
- **Top countries** and **top referrers** as ranked lists.
- Date range picker: 7d / 30d / 90d / 365d.

---

## Implementation Sequence

```
[ ] migrations/0026_analytics.up.sql + down
[ ] sqlc queries + regenerate
[ ] internal/analytics/service.go
[ ] internal/analytics/handler.go
[ ] Wire beacon + API routes into server.go
[ ] Frontend beacon module
[ ] Free analytics UI card
[ ] Pro analytics UI panel (gated by plan)
```

---

## Privacy Notes

- `form_analytics` contains no user-linkable data. Country + device + referrer at day granularity
  cannot re-identify a respondent.
- The beacon endpoint returns `202 Accepted` unconditionally — no error surfaced to the caller,
  no timing side-channel on form existence.
- The query endpoint validates workspace membership before returning any data.
- A per-form "disable analytics" toggle can be added later by checking a flag before calling
  `RecordEvent` — no schema change required.
