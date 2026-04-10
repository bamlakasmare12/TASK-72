# WLPR Fix Check Audit #3 (Static-Only)

## Verdict
- **Overall:** **Partial Pass**
- **Blocking status:** At least one prior **High** issue remains unresolved (`H-01` role-action mismatch), so acceptance is still blocked.

## Scope / Method
- Static-only re-audit focused on previously reported issues: `B-01`, `H-01`, `H-02`, `H-03`, `M-01`, `M-02`, `M-03`.
- No runtime execution (no app start/tests/docker/browser/API calls).
- Evidence references repo files/lines only.

## Issue-by-Issue Resolution Check

### 1) B-01 Core data bootstrap gap (0->1)
- **Status:** **Resolved (static evidence)**
- **Why:** Admin bootstrap create routes now exist for required foundational entities, and docs now describe bootstrap flow via `/api/admin`.
- **Evidence:**
  - `backend/cmd/api/main.go:193`
  - `backend/cmd/api/main.go:194`
  - `backend/cmd/api/main.go:199`
  - `backend/cmd/api/main.go:200`
  - `backend/cmd/api/main.go:201`
  - `backend/internal/handlers/search.go:111`
  - `backend/internal/handlers/learning.go:212`
  - `backend/internal/handlers/procurement.go:415`
  - `backend/internal/handlers/procurement.go:440`
  - `backend/internal/handlers/procurement.go:472`
  - `README.md:48`

### 2) H-01 UI role-action policy mismatch vs backend RBAC
- **Status:** **Not Resolved (still failing)**
- **Why (A):** Procurement users can see/use dispute transition actions in UI, but backend only allows transition for `content_moderator` + `system_admin`.
- **Evidence (A):**
  - UI gates transition actions to procurement/admin: `frontend/src/pages/DisputeQueue.jsx:28`, `frontend/src/pages/DisputeQueue.jsx:254`, `frontend/src/pages/DisputeQueue.jsx:274`
  - UI calls transition endpoint: `frontend/src/pages/DisputeQueue.jsx:96`
  - Backend transition route role gate: `backend/cmd/api/main.go:313`, `backend/cmd/api/main.go:315`
- **Why (B):** Finance analysts can still see invoice matching action in UI, but backend restricts match endpoint to `approver` + `system_admin`.
- **Evidence (B):**
  - Finance route includes finance analysts: `frontend/src/App.jsx:134`
  - Invoice match action shown without role gate: `frontend/src/pages/Reconciliation.jsx:218`, `frontend/src/pages/Reconciliation.jsx:220`
  - Backend match route role gate: `backend/cmd/api/main.go:296`, `backend/cmd/api/main.go:299`

### 3) H-02 Version gate bypass when header omitted
- **Status:** **Resolved (static evidence)**
- **Why:** Missing `X-App-Version` is now treated as `0.0.0`, so compatibility policy still applies.
- **Evidence:**
  - Policy comment and behavior: `backend/internal/middleware/auth_middleware.go:117`, `backend/internal/middleware/auth_middleware.go:130`, `backend/internal/middleware/auth_middleware.go:133`
  - Enforcement path remains active: `backend/internal/middleware/auth_middleware.go:136`, `backend/internal/middleware/auth_middleware.go:169`
  - Added real middleware tests for missing-header cases: `backend/internal/middleware/auth_middleware_security_test.go:57`, `backend/internal/middleware/auth_middleware_security_test.go:91`, `backend/internal/middleware/auth_middleware_security_test.go:117`

### 4) H-03 Dead `/resources/:id` frontend route
- **Status:** **Resolved (static evidence)**
- **Why:** Route and page are now implemented and wired.
- **Evidence:**
  - Catalog links to resource detail: `frontend/src/pages/Catalog.jsx:261`
  - Route exists: `frontend/src/App.jsx:155`
  - Detail page exists and fetches resource: `frontend/src/pages/ResourceDetail.jsx:15`, `frontend/src/pages/ResourceDetail.jsx:27`

### 5) M-01 Role-assignment doc contradiction
- **Status:** **Resolved (static evidence)**
- **Why:** README and backend both state first user is auto-assigned `system_admin`; subsequent users cannot self-assign admin.
- **Evidence:**
  - README behavior: `README.md:45`, `README.md:46`
  - Backend behavior: `backend/internal/handlers/auth.go:175`, `backend/internal/handlers/auth.go:179`

### 6) M-02 Evidence upload pathway ambiguity
- **Status:** **Resolved as documented contract**
- **Why:** System now clearly documents URL-only evidence submission contract (no binary upload), matching service validation.
- **Evidence:**
  - README explicitly states URL-only evidence: `README.md:28`
  - Service enforces URL-based evidence rules: `backend/internal/services/reconciliation.go:194`, `backend/internal/services/reconciliation.go:199`
  - UI labels evidence as URL input: `frontend/src/pages/DisputeQueue.jsx:256`, `frontend/src/pages/DisputeQueue.jsx:258`

### 7) M-03 Security-critical tests were too stub-heavy
- **Status:** **Partially Resolved**
- **What improved:** New middleware security test suite adds stronger checks around real `AppVersionCheck` and RBAC behavior.
- **Evidence improved:** `backend/internal/middleware/auth_middleware_security_test.go:5`, `backend/internal/middleware/auth_middleware_security_test.go:54`, `backend/internal/middleware/auth_middleware_security_test.go:384`
- **Remaining gaps:**
  - Several tests still use custom auth shims instead of full `RequireAuth` session validation chain.
  - Frontend still lacks role-page orchestration tests for `DisputeQueue`/`Reconciliation` action-level gating.
- **Evidence gaps:**
  - Shim pattern in security tests: `backend/internal/middleware/auth_middleware_security_test.go:208`, `backend/internal/middleware/auth_middleware_security_test.go:277`
  - Frontend test set still does not include role-action page tests: `frontend/src/test/ProtectedRoute.test.jsx:1`, `frontend/src/test/Login.test.jsx:1`

## Final Fix-Check Summary
- **Resolved:** `B-01`, `H-02`, `H-03`, `M-01`, `M-02`
- **Partially resolved:** `M-03`
- **Unresolved (blocking):** `H-01`

## Required Next Fixes Before Acceptance
1. Align dispute transition UI actions with backend transition roles (`procurement_specialist` should not call moderator-only transition endpoint, or backend policy must be intentionally expanded).
2. Gate invoice matching UI action to `approver`/`system_admin` only (or change backend role policy and keep consistent).
3. Add role-action integration tests for `DisputeQueue` and `Reconciliation` to prevent regressions.
