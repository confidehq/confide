# Confide API Endpoints

This document enumerates all API endpoints available in the Confide application, as defined in the `internal/server` and various `internal/<module>/handler.go` files.

## Table of Contents
1. [General & Public](#general--public)
2. [Authentication (`/api/auth`)](#authentication-apiauth)
3. [Forms (`/api/forms`)](#forms-apiforms)
4. [Responses (`/api/forms/{formId}/responses`)](#responses-apiformsformidresponses)
5. [Identity (`/api`)](#identity-api)
6. [Workspaces (`/api/workspaces`)](#workspaces-apiworkspaces)
7. [Invitations](#invitations)
8. [Billing (`/api/workspaces/{workspaceId}/billing`)](#billing-apiworkspacesworkspaceidbilling)

---

## General & Public

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/health` | Health check endpoint. Returns status and version. |
| GET | `/api/config` | Returns public configuration (e.g., `formsDomain`). |
| POST | `/api/stripe/webhook` | Stripe webhook for processing subscription updates. |
| POST | `/relay/submit` | Anonymous form submission. CORS-open, rate-limited. |
| GET | `/api/f/{id}/schema` | Public form schema retrieval for respondents. |

---

## Authentication (`/api/auth`)

### Public Auth
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/auth/check-username` | Check if a username is available. |
| POST | `/api/auth/register/begin` | Start the registration process (WebAuthn). |
| POST | `/api/auth/register/finish` | Complete the registration process. |
| POST | `/api/auth/login/begin` | Start the login process (WebAuthn). |
| POST | `/api/auth/login/finish` | Complete the login process. |
| POST | `/api/auth/recover` | Initiate account recovery with a recovery code. |
| POST | `/api/auth/recover/rekey/begin` | Start re-keying the account after recovery. |
| POST | `/api/auth/recover/rekey/finish` | Complete re-keying. |
| GET | `/api/auth/pairing/code/{code}` | Resolve a short pairing code to a token. |
| GET | `/api/auth/pairing/{token}` | Poll the status of a device pairing request. |
| POST | `/api/auth/pairing/{token}/request` | Request pairing from a new device. |
| POST | `/api/auth/pairing/{token}/complete` | Complete pairing on the new device. |

### Authenticated Auth (Requires Session)
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/auth/me` | Get profile info for the currently authenticated user. |
| POST | `/api/auth/logout` | Terminate the current session. |
| GET | `/api/auth/sessions` | List all active sessions for the account. |
| DELETE | `/api/auth/sessions` | Revoke all other active sessions. |
| DELETE | `/api/auth/sessions/{id}` | Revoke a specific session. |
| POST | `/api/auth/reauth/begin` | Start re-authentication (for sensitive actions). |
| POST | `/api/auth/reauth/finish` | Complete re-authentication. |
| POST | `/api/auth/credentials/add/begin` | Start adding a new passkey. |
| POST | `/api/auth/credentials/add/finish` | Complete adding a new passkey. |
| GET | `/api/auth/credentials` | List all registered passkeys/credentials. |
| PATCH | `/api/auth/credentials/{id}` | Rename a credential. |
| DELETE | `/api/auth/credentials/{id}` | Remove a credential. |
| POST | `/api/auth/recovery-code/rotate` | Generate a new set of recovery codes. |
| DELETE | `/api/auth/account` | Permanently delete the user account. |
| POST | `/api/auth/pairing` | Initiate a pairing process from an authorized device. |
| POST | `/api/auth/pairing/{token}/fulfill` | Fulfill a pairing request from an authorized device. |

---

## Forms (`/api/forms`)

Requires authentication.

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/forms` | Create a new form. |
| GET | `/api/forms` | List all forms in a workspace (query `workspaceId` optional). |
| GET | `/api/forms/{id}` | Get detailed configuration for a specific form. |
| PUT | `/api/forms/{id}` | Update the draft schema of a form. |
| POST | `/api/forms/{id}/publish` | Publish the draft schema to make it live. |
| PUT | `/api/forms/{id}/status` | Manually open or close a form. |
| PUT | `/api/forms/{id}/expiration` | Update expiration date, response limits, or TTL. |
| PUT | `/api/forms/{id}/workspace-form-key` | Set or update the workspace-wrapped form key. |
| PUT | `/api/forms/{id}/notification` | Configure PGP-encrypted email notifications. |
| DELETE | `/api/forms/{id}` | Delete a form and all its responses. |
| GET | `/api/forms/{id}/schema-versions/{version}` | Retrieve a specific historical schema version. |

---

## Responses (`/api/forms/{formId}/responses`)

Requires authentication.

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/forms/{formId}/responses` | List responses for a form (supports pagination). |
| GET | `/api/forms/{formId}/responses/{rid}` | Get a specific response's encrypted data. |
| DELETE | `/api/forms/{formId}/responses/{rid}` | Delete a specific response. |

---

## Identity (`/api`)

Requires authentication.

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/account/identity-key` | Get the user's own identity keypair. |
| PUT | `/api/account/identity-key` | Set/update the user's identity keypair. |
| GET | `/api/accounts/{id}/identity-key` | Get another user's public identity key. |

---

## Workspaces (`/api/workspaces`)

Requires authentication.

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/workspaces` | Create a new workspace. |
| GET | `/api/workspaces` | List all workspaces the user is a member of. |
| GET | `/api/workspaces/{id}` | Get workspace details and user's role. |
| PATCH | `/api/workspaces/{id}` | Rename the workspace (Admin+). |
| DELETE | `/api/workspaces/{id}` | Delete the workspace (Owner only). |
| GET | `/api/workspaces/{id}/members` | List members of the workspace. |
| GET | `/api/workspaces/{id}/members/identity-keys` | List public identity keys for all members. |
| GET | `/api/workspaces/{id}/member-key` | Get the caller's own wrapped workspace key. |
| PATCH | `/api/workspaces/{id}/members/{accountId}` | Update a member's role (Admin+). |
| DELETE | `/api/workspaces/{id}/members/{accountId}` | Remove a member from the workspace (Admin+). |
| POST | `/api/workspaces/{id}/member-key` | Grant a wrapped workspace key to a member (Admin+). |
| GET | `/api/workspaces/{id}/pending-key-grants` | List members who need a workspace key (Admin+). |
| GET | `/api/workspaces/{id}/custom-domain` | Get custom domain configuration and status. |
| PUT | `/api/workspaces/{id}/custom-domain` | Configure a custom domain for the workspace. |
| POST | `/api/workspaces/{id}/custom-domain/verify` | Trigger manual verification of domain records. |
| DELETE | `/api/workspaces/{id}/custom-domain` | Remove the custom domain configuration. |

---

## Invitations

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/workspaces/{workspaceId}/invitations` | Invite a user to a workspace (Admin+). |
| GET | `/api/workspaces/{workspaceId}/invitations` | List pending invitations (Admin+). |
| DELETE | `/api/workspaces/{workspaceId}/invitations/{inviteId}` | Revoke a pending invitation (Admin+). |
| GET | `/api/invitations/{token}` | Public: Preview invitation details. |
| POST | `/api/invitations/{token}/accept` | Auth: Accept an invitation. |

---

## Billing (`/api/workspaces/{workspaceId}/billing`)

Requires workspace owner role.

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/workspaces/{workspaceId}/billing` | Get current plan, usage, and billing status. |
| POST | `/api/workspaces/{workspaceId}/billing/subscribe` | Create a Stripe Checkout session for a plan. |
| POST | `/api/workspaces/{workspaceId}/billing/portal` | Create a Stripe Customer Portal session. |
