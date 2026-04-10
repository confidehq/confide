# Workspaces — Design & Implementation Plan

**Status:** Planning  
**Scope:** Multi-phase feature. Adds workspace-based multi-tenancy, collaborative E2E encryption, email-based invitations, role-based access control, and Stripe billing.

---

## Overview

A **Workspace** is the top-level organizational unit in Confide. Every user belongs to at least one workspace. A free account is simply a user who owns a single-member workspace on the free plan — there is no special-casing for solo users.

Key decisions:

- Forms belong to a workspace, not to an individual account.
- The server never sees plaintext form keys — all key distribution is done client-side in the browser.
- Invitations are email-based with short-lived tokens.
- Every existing account gets a personal workspace auto-created during migration.
- Billing is per-workspace via Stripe.

---

## Cryptographic Architecture

### Problem

The current key derivation is deterministic per-user:

```
masterKey  (derived from passkey PRF + prf_salt)
└── formKey = HKDF(masterKey, formId)
    └── formKeypair (X25519) — public key stored server-side
```

This works for solo users but cannot be shared. Another user has no way to derive `formKey` from their own `masterKey`.

### Solution: Identity Keypairs + Per-Member Wrapped Form Keys

Add an **X25519 identity keypair** to each account. The private half is wrapped with the user's `masterKey` and stored on the server. The public half is stored plaintext.

```
masterKey  (unchanged)
├── identityPrivateKey  → wrapped with masterKey (AES-KW), stored server-side
└── identityPublicKey   → stored plaintext server-side (safe to expose)
```

When a form is created in a workspace, the creator wraps `formKey` for each member using ECIES:

```
For each workspace member:
  ephemeral  = X25519.generateKeypair()
  shared     = ECDH(ephemeral.private, member.identityPublicKey)
  wrapKey    = HKDF(shared, "confide-form-key-wrap-v1")
  wrapped    = AES-KW(wrapKey, formKey)
  store → form_member_keys { form_id, account_id, wrapped_form_key, ephemeral_public_key }
```

A member decrypts by reversing:

```
shared   = ECDH(identityPrivateKey, ephemeral_public_key)
wrapKey  = HKDF(shared, "confide-form-key-wrap-v1")
formKey  = AES-KW-unwrap(wrapKey, wrapped_form_key)
```

### Key Distribution on New Member Join

When a new member accepts an invitation, they have no `form_member_keys` entries. The server cannot distribute keys — it only has ciphertext. Instead:

1. Server records the new member in `workspace_members`.
2. Any admin or owner who logs in sees a "pending key grants" notification.
3. Their browser fetches the list of (form, member) pairs missing keys.
4. For each pair: unwrap own `formKey` using own identity key, re-wrap for the new member's identity public key, upload wrapped copy.
5. No real-time coordination is required between admin and new member.

### Existing Account Migration

On first login after deploy, the client detects a missing identity key, generates one (X25519 keypair), wraps the private key with `masterKey`, and uploads both to a new endpoint — silently, before entering the app.

---

## Database Schema

### New Tables

**`workspaces`**
```sql
id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
name             TEXT NOT NULL,
slug             TEXT UNIQUE NOT NULL,
stripe_customer_id         TEXT,
stripe_subscription_id     TEXT,
plan             TEXT NOT NULL DEFAULT 'free',   -- 'free' | 'pro' | 'team'
plan_status      TEXT NOT NULL DEFAULT 'active', -- 'active' | 'trialing' | 'past_due' | 'canceled'
plan_period_end  TIMESTAMPTZ,
created_at       TIMESTAMPTZ NOT NULL DEFAULT now()
```

**`workspace_members`**
```sql
workspace_id  UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
account_id    UUID NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
role          TEXT NOT NULL,  -- 'owner' | 'admin' | 'member' | 'viewer'
joined_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
PRIMARY KEY (workspace_id, account_id)
```

**`workspace_invitations`**
```sql
id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
workspace_id            UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
invited_by_account_id   UUID NOT NULL REFERENCES accounts(id),
email                   TEXT NOT NULL,
role                    TEXT NOT NULL,
token_hash              TEXT NOT NULL UNIQUE,  -- SHA-256 of the raw invite token
expires_at              TIMESTAMPTZ NOT NULL,  -- 72 hours from creation
accepted_at             TIMESTAMPTZ,
created_at              TIMESTAMPTZ NOT NULL DEFAULT now()
```

**`account_identity_keys`**
```sql
account_id                  UUID PRIMARY KEY REFERENCES accounts(id) ON DELETE CASCADE,
identity_public_key          BYTEA NOT NULL,  -- raw 32-byte X25519 public key
wrapped_identity_private_key BYTEA NOT NULL,  -- AES-KW wrapped with masterKey
created_at                  TIMESTAMPTZ NOT NULL DEFAULT now()
```

**`form_member_keys`**
```sql
form_id               UUID NOT NULL REFERENCES forms(id) ON DELETE CASCADE,
account_id            UUID NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
wrapped_form_key      BYTEA NOT NULL,
ephemeral_public_key  BYTEA NOT NULL,  -- 32-byte X25519 ephemeral public key
granted_by_account_id UUID NOT NULL REFERENCES accounts(id),
created_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
PRIMARY KEY (form_id, account_id)
```

### Modified Tables

**`accounts`** — add `email TEXT UNIQUE` (nullable initially; required to receive invitations)

**`forms`** — add `workspace_id UUID NOT NULL REFERENCES workspaces(id)` and `created_by_account_id UUID NOT NULL REFERENCES accounts(id)`. The existing `account_id` column is removed after data migration (see Phase 1).

---

## Roles & Permissions

| Action | Owner | Admin | Member | Viewer |
|---|:---:|:---:|:---:|:---:|
| Delete workspace | ✓ | | | |
| Manage billing | ✓ | | | |
| Invite / remove members | ✓ | ✓ | | |
| Change roles (up to own level) | ✓ | ✓ | | |
| Create / edit / delete forms | ✓ | ✓ | ✓ | |
| View forms & decrypt responses | ✓ | ✓ | ✓ | ✓ |
| Distribute form keys to new members | ✓ | ✓ | | |

Roles are enforced server-side via a `RequireWorkspaceRole` middleware that resolves membership from the session and the workspace context (either an explicit workspace ID param or the workspace that owns the targeted form).

---

## Billing Tiers

| Plan | Members | Forms | Responses/mo | Pricing |
|---|---|---|---|---|
| Free | 1 | 3 | 100 | $0 |
| Pro | 10 | 50 | 2,000 | flat monthly fee |
| Team | Unlimited | Unlimited | Unlimited | flat + metered |

Stripe integration:

- One Stripe Customer per workspace, created at workspace creation time.
- Subscriptions are created/updated via the Stripe API.
- A Stripe Billing Portal session handles self-service plan changes and payment method updates.
- A webhook endpoint (`POST /api/stripe/webhook`) handles `customer.subscription.updated`, `customer.subscription.deleted`, and `invoice.payment_failed` to keep `plan` and `plan_status` in sync.
- The Team plan uses Stripe Metered Billing to report monthly response counts.

---

## Phases

---

### Phase 1 — Database Foundation

**Goal:** Schema in place, existing data migrated, no behavior change for existing users.

**Migrations (in order):**

1. Add `email TEXT UNIQUE` to `accounts` (nullable).
2. Create `workspaces` table.
3. Create `workspace_members` table.
4. Create `workspace_invitations` table.
5. Create `account_identity_keys` table.
6. Create `form_member_keys` table.
7. Add `workspace_id` and `created_by_account_id` to `forms` (nullable initially).
8. Data migration: for each existing account, create a workspace named `"{username}'s Workspace"` with a generated slug, assign the account as `owner`, and set `forms.workspace_id` to that workspace and `forms.created_by_account_id` to `forms.account_id`.
9. Add `NOT NULL` constraints to `forms.workspace_id` and `forms.created_by_account_id`.
10. Drop `forms.account_id`.

**Go changes:**

- Re-run `sqlc generate` after migrations.
- Update all Go model references from `form.AccountID` to `form.WorkspaceID` + `form.CreatedByAccountID`.
- Add workspace auto-creation logic to the account registration path (so new accounts always get a personal workspace).

**Exit criterion:** Existing users can log in, see their forms, and create new forms without any behavior change. All forms are now owned by a workspace.

---

### Phase 2 — Identity Keys

**Goal:** Every account has an X25519 identity keypair stored server-side; the crypto primitives for collaborative key wrapping are implemented.

**Backend API:**

- `GET /api/account/identity-key` — returns `{ identityPublicKey, wrappedIdentityPrivateKey }` for the caller.
- `PUT /api/account/identity-key` — upsert; used on first login after migration or after key rotation.
- `GET /api/accounts/{id}/identity-key` — returns `{ identityPublicKey }` only. Never serves another account's wrapped private key.

**Frontend (`crypto.ts` additions):**

```typescript
// Generate a new identity keypair and wrap the private key with masterKey.
generateIdentityKeypair(masterKey: CryptoKey): Promise<{
  publicKey: Uint8Array,
  wrappedPrivateKey: Uint8Array
}>

// Unwrap the identity private key using masterKey. Call on login; hold result in memory.
unwrapIdentityPrivateKey(
  wrappedPrivateKey: Uint8Array,
  masterKey: CryptoKey
): Promise<CryptoKey>

// Wrap a formKey for a recipient using their identity public key (ECIES).
wrapFormKey(
  formKey: CryptoKey,
  recipientPublicKey: Uint8Array
): Promise<{ wrappedFormKey: Uint8Array, ephemeralPublicKey: Uint8Array }>

// Unwrap a formKey using the caller's identity private key.
unwrapFormKey(
  wrappedFormKey: Uint8Array,
  ephemeralPublicKey: Uint8Array,
  identityPrivateKey: CryptoKey
): Promise<CryptoKey>
```

**Registration flow change:** After the passkey ceremony, generate an identity keypair and upload it in the same request that creates the session (or as an immediate follow-up before the UI loads).

**Login flow change:** After the session is established, fetch `wrappedIdentityPrivateKey`, unwrap it with `masterKey`, store in memory alongside `masterKey`.

**Migration path for existing accounts:** On first login post-deploy, detect a 404 on `GET /api/account/identity-key`, generate a keypair, and upload via `PUT`. This is invisible to the user.

**Exit criterion:** All accounts have `account_identity_keys` entries. `GET /api/accounts/{id}/identity-key` returns a public key for any valid account.

---

### Phase 3 — Workspace Management

**Goal:** Users can create workspaces, switch between them, and manage members.

**Backend API:**

- `POST /api/workspaces` — create a workspace; auto-creates a Stripe Customer; initializes the `free` plan; adds caller as `owner`.
- `GET /api/workspaces` — list workspaces the caller belongs to, including their role.
- `GET /api/workspaces/{id}` — workspace detail and caller's role.
- `PATCH /api/workspaces/{id}` — rename (owner/admin only).
- `DELETE /api/workspaces/{id}` — delete workspace (owner only; must cancel subscription first; fails if any members besides owner remain).
- `GET /api/workspaces/{id}/members` — list members and their roles.
- `PATCH /api/workspaces/{id}/members/{accountId}` — change a member's role (owner/admin only; cannot promote above own role; cannot demote the sole owner).
- `DELETE /api/workspaces/{id}/members/{accountId}` — remove a member; deletes their `form_member_keys` entries.

**Updated form routes:**

All existing `/api/forms` routes require the caller to be a member of the workspace that owns the form. Role enforcement: `create` requires `member` or above; `delete` and `update status` require `member` or above (forms belong to the workspace, not the creator).

**Middleware:**

```go
// RequireWorkspaceRole resolves the workspace from the request context
// (either an explicit ?workspace_id param or the form's workspace_id)
// and enforces a minimum role.
func RequireWorkspaceRole(svc *workspace.Service, minimum Role) func(http.Handler) http.Handler
```

**Frontend:**

- A `currentWorkspace` Svelte store, persisted to `localStorage`.
- Workspace switcher in the sidebar: shows all workspaces, highlights active, links to workspace settings.
- Workspace settings page: rename, member list with role badges, danger zone (delete workspace).
- All form-list and form-detail pages scope their API calls to the current workspace.

**Exit criterion:** A user can create a second workspace, switch to it, create forms in it, and switch back — all scoped correctly.

---

### Phase 4 — Invitation System

**Goal:** Admins can invite people by email; recipients can accept and join the workspace.

**Backend API:**

- `POST /api/workspaces/{id}/invitations` — create an invitation (owner/admin only). Generates a random 32-byte token, stores its SHA-256 hash, sends the invite email, returns the invitation metadata. Enforces the plan's member limit.
- `GET /api/workspaces/{id}/invitations` — list pending (unaccepted, unexpired) invitations (owner/admin only).
- `DELETE /api/workspaces/{id}/invitations/{inviteId}` — revoke an invitation (owner/admin only).
- `GET /api/invitations/{token}` — **public, no auth required**. Resolves token to invitation metadata (workspace name, inviting user's username, role, expiry). Returns 404 if token is invalid, expired, or already accepted.
- `POST /api/invitations/{token}/accept` — **requires auth**. Validates that the caller's email matches the invitation's email (or sets the email on the account if not yet set). Creates the `workspace_members` record. Returns 409 if already a member.

**Email infrastructure:**

New config values:

```
CONFIDE_SMTP_HOST
CONFIDE_SMTP_PORT     (default 587)
CONFIDE_SMTP_USER
CONFIDE_SMTP_PASS
CONFIDE_FROM_EMAIL
```

New internal `mailer` package with a single `SendInvitation(to, workspaceName, inviterUsername, role, link string)` function. Invitation link format: `https://{domain}/invite/{rawToken}`.

**Frontend:**

- `/invite/[token]` route: unauthenticated-accessible page showing workspace name, inviting user, and role. "Accept invitation" button. If the user is not logged in, redirects to `/login?return=/invite/{token}` and resumes after auth.
- Invite modal (in workspace members settings): email field + role selector + "Send invite" button.
- Pending invitations list: shows email, role, expiry, and a revoke button.
- After acceptance, the new workspace appears in the workspace switcher and the user is navigated there.

**Exit criterion:** An admin can invite an email address, the recipient receives an email, follows the link, registers or logs in, and lands in the workspace with the correct role.

---

### Phase 5 — Collaborative Key Distribution

**Goal:** All workspace members can decrypt all workspace forms through client-side key distribution.

**On form creation (frontend change):**

After a form is created (server returns `formId`):

1. Derive `formKey` from `masterKey` (as today).
2. Fetch identity public keys for all current workspace members: `GET /api/workspaces/{id}/members/identity-keys`.
3. For each member (including self), call `wrapFormKey(formKey, member.identityPublicKey)`.
4. Upload all wrapped copies: `POST /api/forms/{formId}/member-keys` with body `[{ accountId, wrappedFormKey, ephemeralPublicKey }]`.

**On form response view (frontend change):**

Instead of deriving `formKey` directly from `masterKey`:

1. `GET /api/forms/{formId}/member-key` — fetches the caller's `form_member_keys` entry.
2. Call `unwrapFormKey(wrappedFormKey, ephemeralPublicKey, identityPrivateKey)` → `formKey`.
3. Proceed to decrypt responses as before.

If the entry is missing, show: *"An admin needs to grant you access to this form's responses."*

**New API endpoints:**

- `GET /api/workspaces/{id}/members/identity-keys` — returns `[{ accountId, identityPublicKey }]` for all members (auth required, member or above).
- `POST /api/forms/{formId}/member-keys` — bulk upsert wrapped form keys (auth required, member or above, must already have a key for this form or be the creator).
- `GET /api/forms/{formId}/member-key` — returns the caller's own `form_member_keys` entry.
- `GET /api/workspaces/{id}/pending-key-grants` — returns `[{ accountId, username, formIds[] }]` for members who are missing keys on one or more workspace forms (owner/admin only).
- `POST /api/workspaces/{id}/key-grants` — bulk upload wrapped keys for multiple (form, member) pairs (owner/admin only).

**Admin key distribution flow (frontend):**

When a workspace loads, owners and admins call `GET /api/workspaces/{id}/pending-key-grants`. If the response is non-empty:

- Show a dismissible banner: *"{Name} joined but can't read {N} form(s) yet — Grant access"*.
- Clicking opens a modal. For each missing (form, member) pair:
  1. Fetch caller's own `form_member_keys` entry for that form and unwrap `formKey`.
  2. Fetch the new member's `identityPublicKey`.
  3. Call `wrapFormKey(formKey, newMember.identityPublicKey)`.
  4. Batch upload via `POST /api/workspaces/{id}/key-grants`.
- On completion, dismiss the banner.

**Exit criterion:** A member invited to a workspace with existing forms can, after an admin completes key distribution, decrypt and read all responses in those forms.

---

### Phase 6 — Billing

**Goal:** Plans are enforced; workspace owners can manage subscriptions through Stripe.

**Dependencies:**

Add `github.com/stripe/stripe-go/v76` to `go.mod`.

New config values:

```
CONFIDE_STRIPE_SECRET_KEY
CONFIDE_STRIPE_WEBHOOK_SECRET
CONFIDE_STRIPE_PRICE_PRO        (Stripe Price ID for Pro plan)
CONFIDE_STRIPE_PRICE_TEAM_FLAT  (Stripe Price ID for Team flat fee)
CONFIDE_STRIPE_PRICE_TEAM_USAGE (Stripe Price ID for Team metered usage)
```

**Backend API:**

- `GET /api/workspaces/{id}/billing` — returns current plan, plan status, member count, form count, and monthly response count (owner only).
- `POST /api/workspaces/{id}/billing/subscribe` — create or update a Stripe subscription to the specified plan (owner only). Returns a Stripe Checkout Session URL for initial payment.
- `POST /api/workspaces/{id}/billing/portal` — create a Stripe Billing Portal session and return its URL (owner only). Used for self-service plan changes, payment methods, and invoice history.
- `POST /api/stripe/webhook` — **public, no auth**. Verifies `Stripe-Signature` header using `CONFIDE_STRIPE_WEBHOOK_SECRET`. Handles:
  - `customer.subscription.updated` → update `plan`, `plan_status`, `plan_period_end`
  - `customer.subscription.deleted` → set `plan = 'free'`, `plan_status = 'canceled'`
  - `invoice.payment_failed` → set `plan_status = 'past_due'`

**Workspace creation change:**

When a workspace is created, call `stripe.Customer.New` to create a Stripe Customer and store `stripe_customer_id`. Initialize `plan = 'free'`, `plan_status = 'active'`.

**Plan enforcement (service-layer checks):**

| Operation | Enforcement |
|---|---|
| Invite a member | Check current member count against plan limit; return 402 if at limit |
| Create a form | Check current form count against plan limit; return 402 if at limit |
| Submit a response (relay) | Check monthly response count against plan limit; return 429 if at limit |

Enforcement is implemented as early-return checks in the relevant service methods, not as middleware, to keep the logic close to the business rules.

**Usage reporting for Team plan:**

A background job (daily, added to the existing scheduler in `main.go`) queries monthly response counts per workspace on the Team plan and reports them to Stripe Metered Billing via `stripe.UsageRecord.New`.

**Frontend:**

- Billing page (owner only, under workspace settings): shows current plan, usage meters (members, forms, responses), upgrade/downgrade buttons, and a "Manage Billing →" link that redirects to the Stripe Portal.
- Plan limit error states: when the API returns 402, show an inline message in the invite modal or form creation flow with a link to the billing page.

**Exit criterion:** A workspace on the free plan cannot exceed 1 member, 3 forms, or 100 responses/month. Upgrading via Stripe Checkout moves the workspace to the Pro plan. The Stripe Portal allows plan changes and payment updates. Subscription lifecycle events are correctly reflected in the database.

---

## What Is Explicitly Out of Scope

- Real-time presence or "who's online" indicators.
- Per-form access control within a workspace (all members see all forms).
- Transferring workspace ownership via the UI (database-level operation only for now).
- OAuth / SSO login (WebAuthn only).
- Audit logs.
- CSV / export features.
- Per-form sharing with external parties (the existing render key already handles public form submission; this refers to response-reading access).
