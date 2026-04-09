# Business Logic Questions Log

Below are high-to-medium ambiguity questions found in the requirement, with current understanding and proposed implementation direction.

1. **Question**: Should all users be allowed to self-register, or must user creation be admin-only after initial bootstrap?
   - **Understanding**: The requirement describes local username/password sign-in and named business roles, but does not explicitly define whether registration is open or controlled.
   - **Solution**: Support open self-registration only for non-privileged roles, auto-assign `system_admin` to the first user, and require admin assignment for privileged role changes.

2. **Question**: For MFA, is setup optional per user, optional per role, or enforceable by policy at tenant/system level?
   - **Understanding**: MFA is described as optional, but security/centralized config suggests enforceability may be needed later.
   - **Solution**: Implement optional MFA per user by default, plus a feature/config flag (`mfa_enforcement`) that can require MFA by role or globally.

3. **Question**: What exact behavior should app version compatibility use during the 14-day grace period (full access, read-only by module, or restricted writes only)?
   - **Understanding**: Requirement says unsupported clients are blocked while allowing read-only access up to 14 days, but does not define which endpoints are considered read-only.
   - **Solution**: Treat `GET/HEAD` as read-only and allow during grace; block mutating methods (`POST/PUT/PATCH/DELETE`) once below min supported version; fully block after grace expires.

4. **Question**: How should role-based invisibility be enforced when a user manually calls hidden endpoints?
   - **Understanding**: Requirement emphasizes menus revealing only permitted actions, but does not explicitly state API error semantics.
   - **Solution**: Keep frontend route/menu hiding, and return `404` for unauthorized protected module endpoints (instead of `403`) to reduce feature discoverability.

5. **Question**: What defines a near-duplicate resource for recommendation deduplication (exact hash match, fuzzy text threshold, or both)?
   - **Understanding**: Requirement asks for dedup across near-duplicates but does not specify algorithm or threshold.
   - **Solution**: Use a two-step approach: exact content hash dedup first, then fuzzy similarity (title+description trigram/cosine) with a configurable threshold for near-duplicate suppression.

6. **Question**: For recommendation diversity (max 40% from one category), does the cap apply before or after filtering unavailable/already-completed items?
   - **Understanding**: Requirement states final carousel composition cap, but ordering of eligibility filters is unspecified.
   - **Solution**: Apply eligibility filtering first (role access, status, completion, duplicates), then enforce the 40% category cap on the final ranked candidate set.

7. **Question**: In learning path completion rules (e.g., 6 required + 2 electives), do electives need to come from the same path only, and can one resource satisfy both required and elective counts across multiple paths?
   - **Understanding**: Requirement implies per-path logic but does not address cross-path reuse semantics.
   - **Solution**: Evaluate completion strictly per path; a resource contributes once per path item mapping; cross-path enrollment remains independent even if the same resource appears elsewhere.

8. **Question**: What is the conflict-resolution strategy for cross-device progress sync when two devices update progress offline and sync later?
   - **Understanding**: Requirement asks for cross-device sync in offline network but does not define merge policy.
   - **Solution**: Use deterministic merge: latest `synced_at` wins for bookmark/status fields, and progress percentage keeps the max value to avoid accidental regressions; log conflict events for audit.

9. **Question**: For dispute evidence uploads, are binary files stored in DB, filesystem, or both (with metadata encrypted)?
   - **Understanding**: Requirement explicitly calls for encryption at rest of sensitive metadata, but does not mandate storage location for blobs.
   - **Solution**: Store evidence files in local file store/object path (offline-safe), keep references in DB, and encrypt sensitive evidence metadata (checksums/identity fields) in DB.

10. **Question**: How strict should taxonomy synonym conflict rules be across statuses (active, pending, rejected)?
    - **Understanding**: Requirement gives example blocking two active synonyms mapping to different canonical tags; pending/rejected behavior is not fully defined.
    - **Solution**: Enforce hard conflict only for `active` synonyms; allow `pending_review` duplicates for moderation workflow; prevent approval if conflict still exists at activation time.

11. **Question**: For variance handling, does “under $5.00 auto-suggest write-off but require Finance approval” mean strict `< 5.00` or `<= 5.00`, and in what currency basis?
    - **Understanding**: Requirement uses example wording that is ambiguous on boundary and multi-currency normalization.
    - **Solution**: Define threshold as absolute variance in settlement currency, default strict `< threshold` (configurable), and always require explicit Finance approval before status changes to settled/write-off-approved.

12. **Question**: For offline export integration, should webhook delivery be synchronous with user request or asynchronous with retry queue and compensation logic?
    - **Understanding**: Requirement mentions offline file drop or LAN webhooks plus scheduled jobs with retry/compensation; synchronous webhook would increase user-facing failures.
    - **Solution**: Return CSV download immediately, trigger sink delivery asynchronously, and use retry with backoff + compensation logging for webhook failures.
