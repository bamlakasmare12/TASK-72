audit_report-1 Fix Check (Static)
Reviewed findings from `.tmp/audit_report-1.md` against current repository state (static-only, no execution).

Overall
Result: 6 / 6 previously reported issues are fixed based on static evidence.

Issue-by-Issue Verification
I-01 (High) - Registration privilege escalation
Previous finding: User registration allowed security-sensitive role escalation risk.
Current status: Fixed
Evidence:
`backend/internal/handlers/auth.go:156` keeps first-user bootstrap path explicit.
`backend/internal/handlers/auth.go:166` and `backend/internal/handlers/auth.go:170` prevent privileged self-assignment for subsequent users.
`backend/internal/models/register.go:25` and `frontend/src/pages/Register.jsx:5` enforce non-admin role constraints in request shape/UI.
Conclusion: Original escalation path remains closed.

I-02 (High) - Dispute evidence metadata protection at rest
Previous finding: Sensitive dispute evidence metadata was not sufficiently protected.
Current status: Fixed
Evidence:
`backend/internal/services/reconciliation.go:197` and `backend/internal/services/reconciliation.go:204` encrypt evidence metadata before persistence.
`backend/internal/repository/procurement_repo.go:665` stores encrypted payloads.
`backend/internal/handlers/procurement.go:519` and `backend/internal/handlers/procurement.go:527` scope decryption to admin-level paths.
Conclusion: Encryption-at-rest and role-scoped exposure controls remain in place.

I-03 (High) - Offline export integration completeness
Previous finding: Export delivery/fallback flow was incomplete.
Current status: Fixed
Evidence:
`backend/internal/services/export_sink.go:18`, `backend/internal/services/export_sink.go:64`, and `backend/internal/services/export_sink.go:161` implement file-drop + LAN webhook export/retry logic.
`backend/internal/handlers/procurement.go:238`, `backend/internal/handlers/procurement.go:253`, and `backend/internal/handlers/procurement.go:295` wire export sink usage into settlement/ledger flows.
`backend/cmd/api/main.go:76` and `backend/cmd/api/main.go:136` initialize and inject export sink dependencies.
Conclusion: Implementation-level export integration remains present.

I-04 (Medium) - Recommendation diversity cap overflow
Previous finding: Deferred refill could exceed diversity cap.
Current status: Fixed
Evidence:
`backend/internal/services/recommendation_worker.go:352` and `backend/internal/services/recommendation_worker.go:388` retain cap checks during refill path.
`backend/internal/services/learning_test.go:64` and `backend/internal/services/learning_test.go:114` cover diversity behavior in service tests.
Conclusion: Refill path now preserves category cap behavior.

I-05 (Medium) - Version compatibility grace-window behavior
Previous finding: Grace/read-only/block transitions were incomplete or inconsistent.
Current status: Fixed
Evidence:
`backend/internal/middleware/auth_middleware.go:113`, `backend/internal/middleware/auth_middleware.go:132`, `backend/internal/middleware/auth_middleware.go:145`, `backend/internal/middleware/auth_middleware.go:151`, and `backend/internal/middleware/auth_middleware.go:163` keep grace handling, read-only mode, and hard-stop boundaries.
Conclusion: Version-gate grace semantics remain implemented.

I-06 (Medium) - Non-admin personal-data masking
Previous finding: Non-admin views exposed sensitive identity fields.
Current status: Fixed
Evidence:
`backend/internal/handlers/procurement.go:503`, `backend/internal/handlers/procurement.go:527`, and `backend/internal/handlers/procurement.go:549` return masked forms for non-admin contexts.
`backend/internal/models/procurement.go:196` and `backend/internal/models/procurement.go:242` define masked DTO variants.
Conclusion: Previously flagged masking gaps remain closed on reviewed paths.

Notes
This is a static fix check only. Runtime verification (for example export sink retry/fallback behavior and full backend integration execution) still requires manual execution in an environment with Go toolchain and database runtime.
Additional non-blocking note: README role-assignment wording remains inconsistent with implementation (`README.md:45`, `backend/internal/handlers/auth.go:170`).
