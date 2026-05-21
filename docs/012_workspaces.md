# Workspaces — Design & Implementation Plan

**Status:** Planning  
**Scope:** Multi-phase feature. Adds workspace-based multi-tenancy, collaborative E2E encryption, token-based invitations, role-based access control, and Stripe billing.

---

## Overview

A **Workspace** is the top-level organizational unit in Confide. Every user belongs to at least one workspace. A free account is simply a user who owns a single-member workspace on the free plan — there is no special-casing for solo users.

Key decisions:

- Forms belong to a workspace, not to an individual account.
- The server never sees plaintext form keys — all key distribution is done client-side in the browser.
- Invitations are token-based with short-lived links sent to an email address. The email is stored only on the invitation, never on the account.
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

### Solution: Identity Keypairs + Per-Member Wrapped Workspace Key

Add an **X25519 identity keypair** to each account. The private half is wrapped with the user's `masterKey` and stored on the server. The public half is stored plaintext.

```
masterKey  (unchanged)
├── identityPrivateKey  → wrapped with masterKey (AES-KW), stored server-side
└── identityPublicKey   → stored plaintext server-side (safe to expose)
```

Each workspace has a single **workspace key** — a random 256-bit symmetric key. All form keys are derived from it. Access to the workspace key means access to all forms in the workspace; there is no per-form access control.

```
workspaceKey  (random 256-bit AES key, generated at workspace creation)
└── formKey = HKDF(workspaceKey, "confide-form-v1" || formId)
    └── used to encrypt/decrypt form schemas and responses
```

The workspace key is wrapped once per member using ECIES with their identity public key:

```
For each workspace member:
  ephemeral  = X25519.generateKeypair()
  shared     = ECDH(ephemeral.private, member.identityPublicKey)
  wrapKey    = HKDF(shared, "confide-workspace-key-wrap-v1")
  wrapped    = AES-KW(wrapKey, workspaceKey)
  store → workspace_member_keys { workspace_id, account_id, wrapped_workspace_key, ephemeral_public_key }
```

A member decrypts by reversing:

```
shared         = ECDH(identityPrivateKey, ephemeral_public_key)
wrapKey        = HKDF(shared, "confide-workspace-key-wrap-v1")
workspaceKey   = AES-KW-unwrap(wrapKey, wrapped_workspace_key)
formKey        = HKDF(workspaceKey, "confide-form-v1" || formId)
```

### Key Distribution on New Member Join

When a new member accepts an invitation, they have no `workspace_member_keys` entry. The server cannot distribute keys — it only has ciphertext. Instead:

1. Server records the new member in `workspace_members`.
2. Any admin or owner who logs in sees a "pending key grant" notification.
3. Their browser unwraps their own `workspaceKey` using their identity key, then wraps it for the new member's identity public key.
4. Upload the single wrapped copy: `POST /api/workspaces/{id}/member-keys`.
5. No real-time coordination required. No per-form enumeration — one key grants access to all forms.

### Existing Account Migration

On first login after deploy, the client detects a missing identity key, generates one (X25519 keypair), wraps the private key with `masterKey`, and uploads both to a new endpoint — silently, before entering the app.

---

## Database Schema

### New Tables

**`workspaces`**
```sql
id                     UUID PRIMARY KEY DEFAULT gen_random_uuid(),
name                   TEXT NOT NULL,
slug                   TEXT UNIQUE NOT NULL,
stripe_customer_id     TEXT UNIQUE,              -- NULL until first upgrade; set lazily on subscribe
stripe_subscription_id TEXT UNIQUE,              -- NULL while on free plan
plan                   TEXT NOT NULL DEFAULT 'free'
                         CHECK (plan IN ('free', 'pro')),
plan_status            TEXT NOT NULL DEFAULT 'active'
                         CHECK (plan_status IN ('active', 'past_due', 'canceled')),
plan_period_end        TIMESTAMPTZ,              -- NULL for free; set from Stripe subscription period
created_at             TIMESTAMPTZ NOT NULL DEFAULT now()
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

**`workspace_member_keys`**
```sql
workspace_id           TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
account_id             TEXT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
wrapped_workspace_key  BYTEA NOT NULL,
ephemeral_public_key   BYTEA NOT NULL,  -- 32-byte X25519 ephemeral public key
granted_by_account_id  TEXT NOT NULL REFERENCES accounts(id),
created_at             TIMESTAMPTZ NOT NULL DEFAULT now(),
PRIMARY KEY (workspace_id, account_id)
```

### Modified Tables

**`accounts`** — no changes. Email is not stored on accounts.

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

| Plan | Workspaces | Members | Forms | Responses/mo | Pricing |
|---|---|---|---|---|---|
| Free | 1 | 1 | Unlimited | Unlimited | $0 |
| Pro | Unlimited | Unlimited | Unlimited | Unlimited | flat rate |

The free plan's single workspace is the personal workspace auto-created at registration. Free users who want a second workspace (e.g. for a team) must upgrade to Pro.

Stripe integration:

- One Stripe Customer per workspace, created at workspace creation time.
- Subscriptions are created/updated via the Stripe API.
- A Stripe Billing Portal session handles self-service plan changes and payment method updates.
- A webhook endpoint (`POST /api/stripe/webhook`) handles `customer.subscription.updated`, `customer.subscription.deleted`, and `invoice.payment_failed` to keep `plan` and `plan_status` in sync.
- Both plans are flat-rate. Response limits are enforced server-side against the local database; no usage data is reported to Stripe.

---

## Phases

---

### Phase 1 — Database Foundation

**Goal:** Schema in place, existing data migrated, no behavior change for existing users.

**Migrations (in order):**

1. Create `workspaces` table.
2. Create `workspace_members` table.
3. Create `workspace_invitations` table.
4. Create `account_identity_keys` table.
5. Create `workspace_member_keys` table.
6. Add `workspace_id` and `created_by_account_id` to `forms` (nullable initially).
7. Data migration: for each existing account, create a workspace named `"Private"` with a generated slug, assign the account as `owner`, and set `forms.workspace_id` to that workspace and `forms.created_by_account_id` to `forms.account_id`.
8. Add `NOT NULL` constraints to `forms.workspace_id` and `forms.created_by_account_id`.
9. Drop `forms.account_id`.

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

// Generate a new workspace key (random 256-bit AES key).
generateWorkspaceKey(): Promise<CryptoKey>

// Wrap a workspaceKey for a recipient using their identity public key (ECIES).
wrapWorkspaceKey(
  workspaceKey: CryptoKey,
  recipientPublicKey: Uint8Array
): Promise<{ wrappedWorkspaceKey: Uint8Array, ephemeralPublicKey: Uint8Array }>

// Unwrap a workspaceKey using the caller's identity private key.
unwrapWorkspaceKey(
  wrappedWorkspaceKey: Uint8Array,
  ephemeralPublicKey: Uint8Array,
  identityPrivateKey: CryptoKey
): Promise<CryptoKey>

// Derive a form-specific key from the workspace key.
deriveFormKey(
  workspaceKey: CryptoKey,
  formId: string
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

- `POST /api/workspaces` — create a workspace; enforces the 1-workspace limit for free-plan accounts (returns 402 if exceeded); initializes `plan = 'free'` in the local database; adds caller as `owner`; accepts the owner's `wrappedWorkspaceKey` and `ephemeralPublicKey` (generated client-side before the request). No Stripe call is made.
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

**Goal:** Admins can invite people by email address; recipients follow a secret link to accept and join the workspace. No email is stored on accounts — possession of the token is sufficient proof of identity.

**Backend API:**

- `POST /api/workspaces/{id}/invitations` — create an invitation (owner/admin only). Generates a random 32-byte token, stores its SHA-256 hash, sends the invite email to the provided address (stored only on the invitation record), returns the invitation metadata. Enforces the plan's member limit.
- `GET /api/workspaces/{id}/invitations` — list pending (unaccepted, unexpired) invitations (owner/admin only).
- `DELETE /api/workspaces/{id}/invitations/{inviteId}` — revoke an invitation (owner/admin only).
- `GET /api/invitations/{token}` — **public, no auth required**. Resolves token to invitation metadata (workspace name, inviting user's username, role, expiry). Returns 404 if token is invalid, expired, or already accepted.
- `POST /api/invitations/{token}/accept` — **requires auth**. Validates that the token is valid and unexpired. Creates the `workspace_members` record. Returns 409 if already a member. No email check is performed — the 256-bit token is the proof of receipt.

**Email infrastructure:**

New config values:

```
CONFIDE_SMTP_HOST
CONFIDE_SMTP_PORT     (default 587)
CONFIDE_SMTP_USER
CONFIDE_SMTP_PASS
CONFIDE_SMTP_SENDER
```

New internal `mailer` package with a single `SendInvitation(to, workspaceName, inviterUsername, role, link string)` function. Invitation link format: `https://{domain}/invite/{rawToken}`.

**Frontend:**

- `/invite/[token]` route: unauthenticated-accessible page showing workspace name, inviting user, and role. "Accept invitation" button. If the user is not logged in, redirects to `/login?return=/invite/{token}` and resumes after auth.
- Invite modal (in workspace members settings): email field + role selector + "Send invite" button.
- Pending invitations list: shows email, role, expiry, and a revoke button.
- After acceptance, the new workspace appears in the workspace switcher and the user is navigated there.

**Exit criterion:** An admin can invite an email address, the recipient receives the link, registers or logs in, follows the link, and lands in the workspace with the correct role. No email is stored on the account at any point.

---

### Phase 5 — Collaborative Key Distribution

**Goal:** All workspace members can decrypt all workspace forms through a single client-side workspace key distribution.

**On workspace creation (frontend change):**

Before calling `POST /api/workspaces`:

1. Call `generateWorkspaceKey()` → `workspaceKey`.
2. Fetch the creator's own `identityPublicKey`.
3. Call `wrapWorkspaceKey(workspaceKey, creator.identityPublicKey)` → `{ wrappedWorkspaceKey, ephemeralPublicKey }`.
4. Include both in the `POST /api/workspaces` body. The server stores them in `workspace_member_keys` for the owner.

**On form creation (frontend change):**

No per-form key work is needed. The workspace key is already in memory (unwrapped at login). The form ID is used to derive the form key client-side:

```
formKey = deriveFormKey(workspaceKey, formId)
```

**On form response view (frontend change):**

Same derivation:

```
workspaceKey = (already in memory from login)
formKey = deriveFormKey(workspaceKey, formId)
```

Decrypt responses as before. No server round-trip for the key.

**New API endpoints:**

- `GET /api/workspaces/{id}/members/identity-keys` — returns `[{ accountId, identityPublicKey }]` for all members (auth required, member or above).
- `GET /api/workspaces/{id}/member-key` — returns the caller's own `workspace_member_keys` entry `{ wrappedWorkspaceKey, ephemeralPublicKey }`.
- `POST /api/workspaces/{id}/member-key` — upsert a wrapped workspace key for a specific member (owner/admin only). Body: `{ accountId, wrappedWorkspaceKey, ephemeralPublicKey }`.
- `GET /api/workspaces/{id}/pending-key-grants` — returns `[{ accountId, username }]` for members who have no `workspace_member_keys` entry (owner/admin only).

**Login flow change:**

After the session is established:

1. Fetch `wrappedIdentityPrivateKey` and `{ wrappedWorkspaceKey, ephemeralPublicKey }` for the current workspace.
2. Unwrap `identityPrivateKey` with `masterKey`.
3. Unwrap `workspaceKey` with `identityPrivateKey`.
4. Hold both `identityPrivateKey` and `workspaceKey` in memory for the session.

**Admin key distribution flow (frontend):**

When a workspace loads, owners and admins call `GET /api/workspaces/{id}/pending-key-grants`. If the response is non-empty:

- Show a dismissible banner: *"{Name} joined but doesn't have workspace access yet — Grant access"*.
- Clicking opens a confirmation. For each pending member:
  1. Fetch their `identityPublicKey`.
  2. Call `wrapWorkspaceKey(workspaceKey, member.identityPublicKey)`.
  3. Upload via `POST /api/workspaces/{id}/member-key`.
- One request per pending member. No form enumeration.

**Exit criterion:** A member invited to a workspace can, after an admin grants them the workspace key, derive and decrypt all form keys and responses. The admin performs one wrap operation per new member regardless of how many forms exist.

---

### Phase 6 — Billing

**Goal:** Plans are enforced; workspace owners can manage subscriptions through Stripe.

**Dependencies:**

Add `github.com/stripe/stripe-go/v76` to `go.mod`.

New config values:

```
CONFIDE_STRIPE_SECRET_KEY
CONFIDE_STRIPE_WEBHOOK_SECRET
CONFIDE_STRIPE_PRICE_PRO   (Stripe Price ID for Pro flat-rate plan)
```

**Privacy principle:** Stripe is contacted only when a workspace actively upgrades to a paid plan. Free workspaces have no Stripe presence. No usage data is ever sent to Stripe — all limits are enforced server-side against the local database.

**Backend API:**

- `GET /api/workspaces/{id}/billing` — returns current plan, plan status, member count, form count, and monthly response count (owner only). All data comes from the local database; no Stripe call is made.
- `POST /api/workspaces/{id}/billing/subscribe` — upgrade or change plan (owner only). If `stripe_customer_id` is NULL, creates a Stripe Customer at this point (lazy creation) passing only `metadata: { workspace_id }` — no name or email. Then creates or updates a Stripe Checkout Session and returns its URL.
- `POST /api/workspaces/{id}/billing/portal` — create a Stripe Billing Portal session and return its URL (owner only). Used for payment method updates and invoice history. Only available once a Stripe Customer exists (i.e., workspace has been on a paid plan).
- `POST /api/stripe/webhook` — **public, no auth**. Verifies `Stripe-Signature` header using `CONFIDE_STRIPE_WEBHOOK_SECRET`. Handles:
  - `customer.subscription.updated` → update `plan`, `plan_status`, `plan_period_end`
  - `customer.subscription.deleted` → set `plan = 'free'`, `plan_status = 'canceled'`
  - `invoice.payment_failed` → set `plan_status = 'past_due'`

**Workspace creation change:**

No Stripe call is made at workspace creation. `stripe_customer_id` starts `NULL`. Initialize `plan = 'free'`, `plan_status = 'active'` in the local database only.

**Lazy Stripe Customer creation:**

```go
// In the subscribe handler, before creating a Checkout Session:
if workspace.StripeCustomerID == nil {
    customer, err := stripe.Customer.New(&stripe.CustomerParams{
        Params: stripe.Params{
            Metadata: map[string]string{"workspace_id": workspace.ID},
        },
    })
    // store customer.ID in workspaces.stripe_customer_id
}
```

No workspace name, owner email, or any other identifying data is passed to Stripe.

**Plan enforcement (service-layer checks):**

| Operation | Enforcement |
|---|---|
| Create a workspace | Check workspace count for the owner's account; free plan is limited to 1 workspace; return 402 if at limit |
| Invite a member | Check current member count against plan limit; return 402 if at limit |
| Create a form | No limit on either plan |
| Submit a response (relay) | No limit on either plan |

Enforcement is implemented as early-return checks in the relevant service methods against the local database — Stripe is never queried at request time.

**Frontend:**

- Billing page (owner only, under workspace settings): shows current plan, usage meters (members, forms, responses), and upgrade/downgrade buttons. If a Stripe Customer exists, a "Manage Billing →" link opens the Stripe Portal for payment method and invoice management.
- Plan limit error states: when the API returns 402, show an inline message in the invite modal or form creation flow with a link to the billing page.
- Stripe.js is never loaded on any app page. Stripe is only encountered on Stripe-hosted Checkout and Portal pages during billing flows.

**Exit criterion:** Free workspaces have no Stripe Customer and no data in Stripe. A user on the free plan cannot create a second workspace (blocked with a 402) and cannot exceed 1 member per workspace. Upgrading via Stripe Checkout lazily creates a Stripe Customer (with only the workspace ID as metadata) and moves the workspace to Pro. Subscription lifecycle events are correctly reflected in the local database. No usage data is reported to Stripe.

---

## What Is Explicitly Out of Scope

- Real-time presence or "who's online" indicators.
- Per-form access control within a workspace (all members see all forms).
- Transferring workspace ownership via the UI (database-level operation only for now).
- OAuth / SSO login (WebAuthn only).
- Audit logs.
- CSV / export features.
- Per-form sharing with external parties (the existing render key already handles public form submission; this refers to response-reading access).
