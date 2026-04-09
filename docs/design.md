# WLPR Portal System Design and Implementation Plan

## 1) System Overview

The WLPR (Workforce Learning & Procurement Reconciliation) Portal is an offline-first, on-prem enterprise application that combines:

- Learning and development workflows (catalog search, taxonomy, learning paths, progress tracking, recommendations)
- Procurement operations (vendors, orders, invoices, reviews, disputes)
- Finance and reconciliation workflows (ledger, settlement lifecycle, variance handling, exports)
- Centralized runtime configuration and feature flags

Primary goals:

- Keep all operations on local infrastructure (no required external cloud dependency)
- Enforce strict role-based access and feature invisibility
- Provide predictable, auditable workflows for compliance-sensitive actions
- Support asynchronous background processing for recommendations and maintenance jobs

## 2) Architecture

### Logical Architecture

The system follows a layered monolith architecture:

1. **Frontend** (`repo/frontend`)
   - React + Vite SPA
   - Route-level role guards and module-based navigation
   - Axios client with auth + version headers
2. **API Backend** (`repo/backend`)
   - Go + Echo REST API
   - Middleware for JWT auth, RBAC, app-version gating, logging, panic recovery
   - Handler -> Service -> Repository layering
3. **Database**
   - PostgreSQL 15 as system-of-record
   - Business constraints implemented with enums, indexes, materialized views, and triggers
4. **Background Jobs**
   - In-process scheduler polling `scheduled_jobs`
   - Retry and compensation handlers for resilience

### Runtime/Deployment Architecture

- `docker-compose.yml` deploys 3 services:
  - `db` (PostgreSQL)
  - `backend` (Go API)
  - `frontend` (Nginx serving built React app)
- Shared `config/config.js` is mounted into both backend and frontend, acting as the single configuration source.

### Key Modules

- **Auth & Identity**: registration bootstrap (first user -> `system_admin`), login, optional MFA setup/verify/disable, session lifecycle.
- **Configuration Center**: key-value configs and feature flags with rollout strategies (`all`, `role_based`, `percentage`, `disabled`).
- **Catalog & Search**: weighted full-text search, trigram similarity, pinyin/synonym-enhanced matching, archive views.
- **Taxonomy Governance**: hierarchical tags, synonym conflicts, review queue with approval/rejection audit trail.
- **Learning**: path enrollment, progress sync, recommendations, CSV export.
- **Procurement & Reviews**: vendor/order/invoice workflows, review/reply/dispute operations.
- **Finance & Reconciliation**: ledger AR/AP, statement comparison, settlement state transitions, export channels.

## 3) Tech Stack

- **Frontend**: React, React Router, Zustand, Axios, Vite, Vitest
- **Backend**: Go 1.22+, Echo, pgx/pgxpool
- **Database**: PostgreSQL 15 with `pgcrypto` and `pg_trgm`
- **Infra**: Docker Compose, Nginx
- **Security primitives**:
  - bcrypt for password hashing
  - AES-256-GCM for encrypted MFA/dispute metadata
  - JWT for stateless API authentication with server-side session checks

## 4) Data Design (Core Schema)

Schema is defined in `repo/backend/init.sql` and includes:

- **Identity & Access**
  - `users`, `roles`, `permissions`, `user_roles`, `role_permissions`, `sessions`
- **Runtime Configuration**
  - `configs`, `feature_flags`, `app_versions`, `scheduled_jobs`, `scheduled_job_runs`
- **Search & Taxonomy**
  - `taxonomy_tags`, `taxonomy_synonyms`, `taxonomy_review_queue`, `resources`, `resource_tags`
  - Materialized views: `resource_archive_monthly`, `resource_archive_by_tag`
- **Learning Domain**
  - `learning_paths`, `learning_path_items`, `user_enrollments`, `user_progress`, `resource_events`, `recommendations`
- **Procurement & Finance**
  - `vendors`, `procurement_orders`, `order_line_items`, `invoices`, `ledger_entries`, `settlements`, `billing_rules`
- **Review/Dispute Governance**
  - `vendor_reviews`, `merchant_replies`, `disputes`
- **Auditability**
  - `audit_log`

Important DB-enforced business rules:

- Synonym conflict trigger to prevent duplicate active mappings to different canonical tags
- Dispute state-machine trigger to reject invalid transition paths
- Invoice variance auto-classification trigger using configurable threshold
- Search vector and content hash triggers for indexing and dedup support

## 5) Security and Access Model

- Role-based API protection via middleware in route groups
- Restricted module responses intentionally return non-discoverable behavior in UX/API design
- Session constraints:
  - idle timeout (default 15m)
  - max lifetime (default 8h)
- Account lockout after repeated failed attempts
- Version gate middleware using `X-App-Version` and min-supported config

## 6) Reliability and Operations

- Health endpoint: `GET /api/health`
- Scheduler-driven maintenance and analytics jobs:
  - recommendation rebuild
  - session cleanup
  - archive refresh
- Retry + compensation handlers for failed scheduled jobs
- Export paths support both direct CSV response and optional sink delivery (file-drop/webhook)

## 7) Implementation Plan

### Phase 1: Baseline Stabilization

1. Validate configuration contracts
   - Verify all required keys in `config/config.js`
   - Add startup validation checklist for deploy pipelines
2. Confirm RBAC coverage
   - Ensure every API route belongs to an explicit role group
   - Add missing test cases for negative-role access paths
3. Validate bootstrap flows
   - First-user admin assignment
   - Normal registration path and admin role assignment

**Exit criteria**: backend boots cleanly, auth+RBAC tests pass, no ambiguous route protection.

### Phase 2: Domain Hardening

1. Search and taxonomy quality
   - Validate pinyin/synonym flag behavior combinations
   - Review archive-refresh cadence and fallback behavior
2. Learning workflow hardening
   - Enrollment idempotency checks
   - Progress update validation (status/pct consistency)
3. Reconciliation/dispute correctness
   - Expand state transition tests for settlement/dispute edge cases
   - Validate invoice match + variance outcomes against threshold configs

**Exit criteria**: deterministic behavior for all state transitions and major read/write workflows.

### Phase 3: Observability and Operability

1. Structured logging conventions
   - Standardize module/action/user/session fields in logs
2. Runbook + incident readiness
   - Document scheduler troubleshooting and stuck-job recovery
   - Document export sink failure handling
3. Data lifecycle governance
   - Define retention/cleanup plan for `audit_log`, `scheduled_job_runs`, and event tables

**Exit criteria**: operational runbook exists and is tested in staging-like environment.

### Phase 4: Delivery Readiness

1. Regression suite execution
   - Backend unit/integration tests
   - Frontend test + production build
   - Full docker smoke/e2e path
2. Security review
   - Rotate dev secrets for production
   - Validate encryption key/JWT key management process
3. Documentation completion
   - API contract and role matrix kept aligned to code routes
   - Deployment and rollback checklist finalized

**Exit criteria**: release checklist completed and reproducible deployment validated.

## 8) Risks and Mitigations

- **Risk**: Feature flags/config drift between DB and mounted config
  - **Mitigation**: startup config audit and admin dashboard drift indicator
- **Risk**: Scheduler running in single process (no distributed coordination)
  - **Mitigation**: single-backend-instance policy or future leader-lock strategy
- **Risk**: Large table growth (events/audit/runs) impacting query performance
  - **Mitigation**: retention policy + indexes review + periodic archiving
- **Risk**: Role leakage via frontend-only checks
  - **Mitigation**: backend enforcement remains source of truth; expand RBAC API tests

## 9) Success Metrics

- Authentication success rate and lockout false-positive rate
- Search latency and relevance quality (result click-through/completion)
- Reconciliation automation ratio (auto-resolution vs manual)
- Dispute resolution cycle time
- Scheduler success rate and compensation frequency
