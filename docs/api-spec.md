# WLPR Portal API Specification

## Base

- Base URL: `/api`
- Content type: `application/json` unless noted (CSV exports)
- Auth: `Authorization: Bearer <jwt>` for protected endpoints
- Client version header: `X-App-Version: <semver>`

## Conventions

- Standard success codes:
  - `200` OK (read/update)
  - `201` Created (create operations)
- Typical error codes:
  - `400` invalid request
  - `401` unauthenticated/invalid credentials
  - `404` not found (also used for hidden role-restricted routes)
  - `409` state transition conflict
  - `500` server error

---

## 1) Health

| Method | Path | Auth | Description |
|---|---|---|---|
| GET | `/health` | No | Health probe |

Response example:

```json
{ "status": "ok" }
```

## 2) Authentication and User Admin

| Method | Path | Auth | Description |
|---|---|---|---|
| POST | `/auth/register` | No | Register user (first user becomes `system_admin`) |
| POST | `/auth/login` | No | Login with username/password |
| POST | `/auth/mfa/verify` | No | Complete MFA login challenge |
| POST | `/auth/logout` | Yes | Revoke current session |
| GET | `/auth/me` | Yes | Return auth claims/user context |
| POST | `/auth/mfa/setup` | Yes | Initiate MFA setup |
| POST | `/auth/mfa/confirm` | Yes | Confirm MFA with TOTP code |
| POST | `/auth/mfa/disable` | Yes | Disable MFA for current user |
| GET | `/admin/users` | Yes (`system_admin`) | List users |
| POST | `/admin/users/assign-role` | Yes (`system_admin`) | Assign role to user |

### Request examples

Register:

```json
{
  "username": "jdoe",
  "email": "jdoe@example.local",
  "password": "StrongPass123",
  "display_name": "Jane Doe",
  "role": "learner",
  "job_family": "engineering",
  "department": "platform",
  "cost_center": "CC-101"
}
```

Login:

```json
{
  "username": "jdoe",
  "password": "StrongPass123"
}
```

MFA verify:

```json
{
  "code": "123456",
  "session_id": "uuid-session-id"
}
```

Assign role:

```json
{
  "user_id": 42,
  "role": "finance_analyst"
}
```

## 3) Configuration and Feature Flags

| Method | Path | Auth | Description |
|---|---|---|---|
| GET | `/config/all` | Yes (`system_admin`) | Get all configs |
| GET | `/config/:key` | Yes (`system_admin`) | Get one config |
| PUT | `/config/:key` | Yes (`system_admin`) | Update config value |
| GET | `/config/flags` | Yes (`system_admin`) | Get all feature flags |
| GET | `/config/flags/:key` | Yes (`system_admin`) | Get one flag |
| PUT | `/config/flags/:key` | Yes (`system_admin`) | Update one flag |
| GET | `/config/flags/:key/check` | Yes (`system_admin`) | Check flag enablement |
| GET | `/flags/:key/check` | Yes | Check flag for current user context |

Update config request:

```json
{ "value": "900" }
```

Update flag request:

```json
{
  "enabled": true,
  "rollout_strategy": "role_based",
  "rollout_percentage": 0,
  "allowed_roles": [1, 3]
}
```

## 4) Catalog, Search, and Taxonomy

### Search/Catalog

| Method | Path | Auth | Description |
|---|---|---|---|
| GET | `/search` | Yes (`learner`, `content_moderator`, `system_admin`) | Search resources |
| GET | `/resources/:id` | Yes (`learner`, `content_moderator`, `system_admin`) | Resource detail |
| GET | `/archives` | Yes (`learner`, `content_moderator`, `system_admin`) | Archive aggregates |

Search query params:

- `q` string
- `categories` comma-separated ints
- `tags` comma-separated ints
- `date_from`, `date_to` as `YYYY-MM-DD`
- `difficulty`, `type`, `sort_by` (`relevance|popularity|recent`)
- `page` (default 1), `page_size` (default 20)

### Taxonomy

| Method | Path | Auth | Description |
|---|---|---|---|
| GET | `/taxonomy/tags` | Yes (`learner`, `content_moderator`, `system_admin`) | List tags |
| GET | `/taxonomy/tags/hierarchy` | Yes (`learner`, `content_moderator`, `system_admin`) | Tag hierarchy |
| GET | `/taxonomy/tags/:id` | Yes (`learner`, `content_moderator`, `system_admin`) | Tag detail |
| GET | `/taxonomy/synonyms/:tag_id` | Yes (`learner`, `content_moderator`, `system_admin`) | Synonyms by canonical tag |
| POST | `/taxonomy/tags` | Yes (`content_moderator`, `system_admin`) | Create pending tag |
| POST | `/taxonomy/synonyms` | Yes (`content_moderator`, `system_admin`) | Create pending synonym |
| GET | `/taxonomy/review-queue` | Yes (`content_moderator`, `system_admin`) | Pending review items |
| GET | `/taxonomy/review-queue/audit` | Yes (`content_moderator`, `system_admin`) | Full review history |
| POST | `/taxonomy/review-queue/approve` | Yes (`content_moderator`, `system_admin`) | Approve review item |
| POST | `/taxonomy/review-queue/reject` | Yes (`content_moderator`, `system_admin`) | Reject review item |

Create tag request:

```json
{
  "name": "Data Analysis",
  "tag_type": "skill",
  "parent_id": 10,
  "pinyin": "shu ju fen xi",
  "description": "Skill taxonomy tag"
}
```

Create synonym request:

```json
{
  "term": "data analytics",
  "canonical_tag_id": 55
}
```

Review action request:

```json
{
  "review_item_id": 101,
  "action": "approve",
  "decision_notes": "Looks correct"
}
```

## 5) Learning

| Method | Path | Auth | Description |
|---|---|---|---|
| GET | `/learning/paths` | Yes (`learner`, `content_moderator`, `system_admin`) | List learning paths |
| GET | `/learning/paths/:id` | Yes (`learner`, `content_moderator`, `system_admin`) | Path detail |
| GET | `/learning/recommendations` | Yes (`learner`, `content_moderator`, `system_admin`) | Personalized recommendations |
| POST | `/learning/enroll` | Yes (`learner`, `content_moderator`, `system_admin`) | Enroll current user in path |
| DELETE | `/learning/enroll/:path_id` | Yes (`learner`, `content_moderator`, `system_admin`) | Drop enrollment |
| GET | `/learning/enrollments` | Yes (`learner`, `content_moderator`, `system_admin`) | List own enrollments |
| GET | `/learning/enrollments/:path_id` | Yes (`learner`, `content_moderator`, `system_admin`) | Enrollment detail |
| PUT | `/learning/progress` | Yes (`learner`, `content_moderator`, `system_admin`) | Update progress |
| GET | `/learning/progress` | Yes (`learner`, `content_moderator`, `system_admin`) | Get progress (optional `path_id`) |
| GET | `/learning/export` | Yes (`learner`, `content_moderator`, `system_admin`) | Export learning CSV |

Enroll request:

```json
{ "path_id": 7 }
```

Update progress request:

```json
{
  "resource_id": 55,
  "path_id": 7,
  "status": "in_progress",
  "progress_pct": 45,
  "time_spent_mins": 30,
  "last_position": "00:12:18"
}
```

## 6) Procurement, Finance, Reconciliation

### Procurement Read/Write

| Method | Path | Auth | Description |
|---|---|---|---|
| GET | `/procurement/vendors` | Yes (`procurement_specialist`, `approver`, `finance_analyst`, `system_admin`) | List vendors |
| GET | `/procurement/orders` | Yes (same as above) | List orders |
| GET | `/procurement/orders/:id` | Yes (same as above) | Order detail |
| PUT | `/procurement/orders/:id/approve` | Yes (`approver`, `system_admin`) | Approve order |
| GET | `/procurement/invoices` | Yes (`procurement_specialist`, `approver`, `finance_analyst`, `system_admin`) | List invoices |
| GET | `/procurement/invoices/:id` | Yes (same as above) | Invoice detail |
| POST | `/procurement/invoices/match` | Yes (`approver`, `system_admin`) | Match invoice to order |
| GET | `/procurement/billing-rules` | Yes (`procurement_specialist`, `approver`, `finance_analyst`, `system_admin`) | Billing rules |

Match invoice request:

```json
{
  "invoice_id": 123,
  "order_id": 456
}
```

### Reviews and Disputes

| Method | Path | Auth | Description |
|---|---|---|---|
| GET | `/procurement/reviews` | Yes (`procurement_specialist`, `system_admin`) | List reviews |
| POST | `/procurement/reviews` | Yes (`procurement_specialist`, `system_admin`) | Create review |
| POST | `/procurement/reviews/reply` | Yes (`procurement_specialist`, `system_admin`) | Create merchant reply |
| GET | `/procurement/disputes` | Yes (`procurement_specialist`, `content_moderator`, `approver`, `system_admin`) | List disputes |
| GET | `/procurement/disputes/:id` | Yes (`procurement_specialist`, `content_moderator`, `approver`, `system_admin`) | Dispute detail |
| POST | `/procurement/disputes` | Yes (`procurement_specialist`, `system_admin`) | Create dispute |
| POST | `/procurement/disputes/transition` | Yes (`content_moderator`, `system_admin`) | Advance dispute state |

Create review request:

```json
{
  "vendor_id": 8,
  "order_id": 110,
  "rating": 4,
  "title": "On-time delivery",
  "body": "Delivery quality was good.",
  "image_urls": ["evidence1.png"]
}
```

Create dispute request:

```json
{
  "review_id": 22,
  "vendor_id": 8,
  "reason": "Evidence disputed"
}
```

Dispute transition request:

```json
{
  "dispute_id": 22,
  "action": "arbitrate",
  "evidence_urls": ["proof.pdf"],
  "evidence_metadata": "checksum=abc123",
  "merchant_response": "Vendor response text",
  "arbitration_notes": "Needs disclaimer",
  "arbitration_outcome": "disclaimer"
}
```

### Finance and Settlement

| Method | Path | Auth | Description |
|---|---|---|---|
| GET | `/procurement/ledger` | Yes (`finance_analyst`, `system_admin`) | List ledger entries |
| POST | `/procurement/ledger` | Yes (`finance_analyst`, `system_admin`) | Create ledger entry |
| GET | `/procurement/cost-allocation` | Yes (`finance_analyst`, `system_admin`) | Cost allocation report |
| POST | `/procurement/reconciliation/compare` | Yes (`finance_analyst`, `system_admin`) | Compare statement vs ledger |
| POST | `/procurement/settlements` | Yes (`finance_analyst`, `system_admin`) | Create settlement |
| GET | `/procurement/settlements` | Yes (`finance_analyst`, `approver`, `system_admin`) | List settlements |
| POST | `/procurement/settlements/transition` | Yes (`finance_analyst`, `approver`, `system_admin`) | Transition settlement |
| GET | `/procurement/export/ledger` | Yes (`finance_analyst`, `system_admin`) | Export ledger CSV |
| GET | `/procurement/export/settlements` | Yes (`finance_analyst`, `system_admin`) | Export settlements CSV |

Create ledger entry request:

```json
{
  "entry_type": "AP",
  "reference_type": "invoice",
  "reference_id": 123,
  "vendor_id": 8,
  "amount": 1500.25,
  "currency": "USD",
  "department": "operations",
  "cost_center": "CC-201",
  "description": "Invoice posting"
}
```

Compare statement request:

```json
{
  "vendor_id": 8,
  "statement_total": 10000.00,
  "period_start": "2026-01-01",
  "period_end": "2026-01-31"
}
```

Settlement transition request:

```json
{
  "settlement_id": 15,
  "action": "settle",
  "notes": "Approved after variance review"
}
```

---

## 7) Response Shape Notes

- Most list endpoints return arrays of strongly typed domain objects.
- CSV endpoints return `text/csv` with `Content-Disposition: attachment`.
- Auth `login` may return MFA challenge (`requires_mfa=true`) prior to token issuance.
- Non-admin dispute/review reads may return masked fields by design.

## 8) Role Matrix (High Level)

- `system_admin`: full API surface
- `content_moderator`: taxonomy governance + dispute arbitration + learning access
- `learner`: learning/search modules
- `procurement_specialist`: procurement read/write + dispute creation
- `approver`: procurement approvals + settlement transitions/read
- `finance_analyst`: ledger/reconciliation/settlement and export operations
