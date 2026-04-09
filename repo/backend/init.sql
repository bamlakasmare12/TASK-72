-- Workforce Learning & Procurement Reconciliation Portal
-- Database Initialization Script
-- PostgreSQL 15+

CREATE EXTENSION IF NOT EXISTS pgcrypto;
CREATE EXTENSION IF NOT EXISTS pg_trgm;

-- ============================================================
-- ENUM TYPES
-- ============================================================

CREATE TYPE session_status AS ENUM ('active', 'expired', 'revoked');
CREATE TYPE flag_rollout_strategy AS ENUM ('all', 'role_based', 'percentage', 'disabled');
CREATE TYPE config_value_type AS ENUM ('string', 'number', 'boolean', 'json');

-- ============================================================
-- ROLES & PERMISSIONS
-- ============================================================

CREATE TABLE roles (
    id          SERIAL PRIMARY KEY,
    name        VARCHAR(50) UNIQUE NOT NULL,
    description TEXT,
    created_at  TIMESTAMPTZ DEFAULT NOW(),
    updated_at  TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE permissions (
    id          SERIAL PRIMARY KEY,
    code        VARCHAR(100) UNIQUE NOT NULL,
    description TEXT,
    module      VARCHAR(50) NOT NULL,
    created_at  TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE role_permissions (
    role_id       INT NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    permission_id INT NOT NULL REFERENCES permissions(id) ON DELETE CASCADE,
    PRIMARY KEY (role_id, permission_id)
);

-- ============================================================
-- USERS & AUTH
-- ============================================================

CREATE TABLE users (
    id                  SERIAL PRIMARY KEY,
    username            VARCHAR(100) UNIQUE NOT NULL,
    email               VARCHAR(255) UNIQUE NOT NULL,
    password_hash       TEXT NOT NULL,
    display_name        VARCHAR(200) NOT NULL,
    mfa_enabled         BOOLEAN DEFAULT FALSE,
    mfa_secret_enc      BYTEA,                          -- AES-256 encrypted TOTP secret
    mfa_recovery_enc    BYTEA,                          -- AES-256 encrypted recovery codes
    job_family          VARCHAR(100),
    department          VARCHAR(100),
    cost_center         VARCHAR(50),
    is_active           BOOLEAN DEFAULT TRUE,
    failed_login_count  INT DEFAULT 0,
    locked_until        TIMESTAMPTZ,
    last_login_at       TIMESTAMPTZ,
    created_at          TIMESTAMPTZ DEFAULT NOW(),
    updated_at          TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE user_roles (
    user_id    INT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role_id    INT NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    granted_at TIMESTAMPTZ DEFAULT NOW(),
    granted_by INT REFERENCES users(id),
    PRIMARY KEY (user_id, role_id)
);

CREATE TABLE sessions (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         INT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash      TEXT NOT NULL,
    status          session_status DEFAULT 'active',
    ip_address      INET,
    user_agent      TEXT,
    created_at      TIMESTAMPTZ DEFAULT NOW(),
    last_active_at  TIMESTAMPTZ DEFAULT NOW(),
    expires_at      TIMESTAMPTZ NOT NULL,
    idle_timeout_s  INT DEFAULT 900,        -- 15 minutes
    max_lifetime_s  INT DEFAULT 28800       -- 8 hours
);

CREATE INDEX idx_sessions_user ON sessions(user_id);
CREATE INDEX idx_sessions_token ON sessions(token_hash);
CREATE INDEX idx_sessions_status ON sessions(status) WHERE status = 'active';

-- ============================================================
-- CONFIGURATION CENTER
-- ============================================================

CREATE TABLE feature_flags (
    id                  SERIAL PRIMARY KEY,
    key                 VARCHAR(100) UNIQUE NOT NULL,
    description         TEXT,
    enabled             BOOLEAN DEFAULT FALSE,
    rollout_strategy    flag_rollout_strategy DEFAULT 'disabled',
    rollout_percentage  INT DEFAULT 0 CHECK (rollout_percentage BETWEEN 0 AND 100),
    allowed_roles       INT[],                          -- role IDs for role_based strategy
    created_by          INT REFERENCES users(id),
    created_at          TIMESTAMPTZ DEFAULT NOW(),
    updated_at          TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE configs (
    id          SERIAL PRIMARY KEY,
    key         VARCHAR(200) UNIQUE NOT NULL,
    value       TEXT NOT NULL,
    value_type  config_value_type DEFAULT 'string',
    module      VARCHAR(50),
    description TEXT,
    updated_by  INT REFERENCES users(id),
    created_at  TIMESTAMPTZ DEFAULT NOW(),
    updated_at  TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE app_versions (
    id                  SERIAL PRIMARY KEY,
    version             VARCHAR(20) NOT NULL,
    min_supported       VARCHAR(20) NOT NULL,
    force_update        BOOLEAN DEFAULT FALSE,
    read_only_grace_days INT DEFAULT 14,
    released_at         TIMESTAMPTZ DEFAULT NOW(),
    created_at          TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE scheduled_jobs (
    id              SERIAL PRIMARY KEY,
    name            VARCHAR(100) UNIQUE NOT NULL,
    cron_expr       VARCHAR(50) NOT NULL,
    handler         VARCHAR(200) NOT NULL,
    enabled         BOOLEAN DEFAULT TRUE,
    last_run_at     TIMESTAMPTZ,
    last_status     VARCHAR(20),
    retry_count     INT DEFAULT 0,
    max_retries     INT DEFAULT 3,
    created_at      TIMESTAMPTZ DEFAULT NOW(),
    updated_at      TIMESTAMPTZ DEFAULT NOW()
);

-- ============================================================
-- AUDIT LOG
-- ============================================================

CREATE TABLE audit_log (
    id          BIGSERIAL PRIMARY KEY,
    user_id     INT REFERENCES users(id),
    action      VARCHAR(100) NOT NULL,
    module      VARCHAR(50),
    entity_type VARCHAR(50),
    entity_id   INT,
    old_value   JSONB,
    new_value   JSONB,
    ip_address  INET,
    created_at  TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_audit_user ON audit_log(user_id);
CREATE INDEX idx_audit_action ON audit_log(action);
CREATE INDEX idx_audit_entity ON audit_log(entity_type, entity_id);

-- ============================================================
-- SEED DATA
-- ============================================================

-- Roles
INSERT INTO roles (name, description) VALUES
    ('system_admin',            'Full system access'),
    ('content_moderator',       'Manage learning content and taxonomy'),
    ('learner',                 'Access learning paths and resources'),
    ('procurement_specialist',  'Manage vendor orders and reviews'),
    ('approver',                'Approve procurement and finance items'),
    ('finance_analyst',         'Reconciliation, settlements, cost allocation');

-- Permissions
INSERT INTO permissions (code, description, module) VALUES
    -- Auth & Admin
    ('admin.users.manage',          'Create/edit/disable users',                'admin'),
    ('admin.roles.manage',          'Assign roles and permissions',             'admin'),
    ('admin.config.manage',         'Manage feature flags and configs',         'admin'),
    ('admin.audit.view',            'View audit logs',                          'admin'),
    -- Learning
    ('learning.library.view',       'Browse learning library',                  'learning'),
    ('learning.path.enroll',        'Enroll in learning paths',                 'learning'),
    ('learning.path.manage',        'Create/edit learning paths',               'learning'),
    ('learning.progress.view_own',  'View own progress',                        'learning'),
    ('learning.progress.view_all',  'View all learner progress',                'learning'),
    ('learning.content.moderate',   'Moderate learning resources',              'learning'),
    -- Procurement
    ('procurement.orders.view',     'View procurement orders',                  'procurement'),
    ('procurement.orders.manage',   'Create/edit procurement orders',           'procurement'),
    ('procurement.reviews.manage',  'Manage vendor reviews and disputes',       'procurement'),
    ('procurement.approve',         'Approve procurement requests',             'procurement'),
    -- Finance
    ('finance.reconciliation.view', 'View reconciliation dashboards',           'finance'),
    ('finance.reconciliation.manage','Execute reconciliation actions',           'finance'),
    ('finance.settlement.manage',   'Manage settlements and write-offs',        'finance'),
    ('finance.reports.export',      'Export financial reports',                  'finance');

-- Role-Permission Mappings
-- system_admin gets everything
INSERT INTO role_permissions (role_id, permission_id)
    SELECT 1, id FROM permissions;

-- content_moderator
INSERT INTO role_permissions (role_id, permission_id)
    SELECT 2, id FROM permissions WHERE code IN (
        'learning.library.view', 'learning.path.manage',
        'learning.content.moderate', 'learning.progress.view_all'
    );

-- learner
INSERT INTO role_permissions (role_id, permission_id)
    SELECT 3, id FROM permissions WHERE code IN (
        'learning.library.view', 'learning.path.enroll', 'learning.progress.view_own'
    );

-- procurement_specialist
INSERT INTO role_permissions (role_id, permission_id)
    SELECT 4, id FROM permissions WHERE code IN (
        'procurement.orders.view', 'procurement.orders.manage', 'procurement.reviews.manage'
    );

-- approver
INSERT INTO role_permissions (role_id, permission_id)
    SELECT 5, id FROM permissions WHERE code IN (
        'procurement.orders.view', 'procurement.approve',
        'finance.reconciliation.view'
    );

-- finance_analyst
INSERT INTO role_permissions (role_id, permission_id)
    SELECT 6, id FROM permissions WHERE code IN (
        'finance.reconciliation.view', 'finance.reconciliation.manage',
        'finance.settlement.manage', 'finance.reports.export'
    );

-- No seeded users: the first user to register is auto-assigned system_admin.
-- All subsequent users register normally and are assigned a role by an admin.

-- Default configs
INSERT INTO configs (key, value, value_type, module, description) VALUES
    ('session.idle_timeout_seconds',    '900',      'number',   'auth',     'Idle timeout in seconds'),
    ('session.max_lifetime_seconds',    '28800',    'number',   'auth',     'Max session lifetime in seconds'),
    ('variance.auto_writeoff_threshold','5.00',     'number',   'finance',  'Auto-suggest write-off threshold in USD'),
    ('recommendation.max_category_pct', '40',       'number',   'learning', 'Max % of carousel from single category'),
    ('app.min_supported_version',       '1.0.0',    'string',   'system',   'Minimum supported client version'),
    ('app.read_only_grace_days',        '14',       'number',   'system',   'Days of read-only access for old clients'),
    ('export.file_drop_dir',            '',         'string',   'finance',  'Directory for offline file-drop exports (empty = disabled)'),
    ('export.webhook_url',              '',         'string',   'finance',  'LAN webhook URL for export delivery (empty = disabled)'),
    ('export.max_retries',              '3',        'number',   'finance',  'Max retry attempts for webhook delivery'),
    ('export.retry_delay_seconds',      '5',        'number',   'finance',  'Seconds between webhook retry attempts');

-- Scheduled jobs
INSERT INTO scheduled_jobs (name, cron_expr, handler, enabled, max_retries) VALUES
    ('recommendation_rebuild', '0 2 * * *',  'recommendation_worker', TRUE, 3),
    ('session_cleanup',        '*/15 * * * *', 'session_cleanup',     TRUE, 3),
    ('archive_refresh',        '0 3 * * *',  'archive_refresh',       TRUE, 3);

-- Default feature flags
INSERT INTO feature_flags (key, description, enabled, rollout_strategy) VALUES
    ('mfa_enforcement',         'Require MFA for all users',    FALSE, 'disabled'),
    ('learning_recommendations','Enable AI recommendations',    TRUE,  'all'),
    ('procurement_disputes',    'Enable dispute workflow',      TRUE,  'all'),
    ('pinyin_search',           'Enable Pinyin matching',       TRUE,  'all'),
    ('synonym_search',          'Enable synonym expansion',     TRUE,  'all');

-- ============================================================
-- TAXONOMY: Hierarchical Job/Skill Tags
-- ============================================================

CREATE TYPE taxonomy_type AS ENUM ('job', 'skill', 'category', 'topic');
CREATE TYPE synonym_status AS ENUM ('active', 'pending_review', 'rejected');
CREATE TYPE taxonomy_review_action AS ENUM ('approve', 'reject', 'merge');

CREATE TABLE taxonomy_tags (
    id              SERIAL PRIMARY KEY,
    name            VARCHAR(200) NOT NULL,
    slug            VARCHAR(200) UNIQUE NOT NULL,
    tag_type        taxonomy_type NOT NULL,
    parent_id       INT REFERENCES taxonomy_tags(id) ON DELETE SET NULL,
    canonical_id    INT REFERENCES taxonomy_tags(id) ON DELETE SET NULL,
    pinyin          VARCHAR(500),                   -- pre-computed pinyin for Chinese matching
    description     TEXT,
    is_canonical    BOOLEAN DEFAULT TRUE,
    level           INT DEFAULT 0,                  -- depth in hierarchy
    sort_order      INT DEFAULT 0,
    usage_count     INT DEFAULT 0,
    status          VARCHAR(20) DEFAULT 'active',   -- active, pending, rejected
    created_by      INT REFERENCES users(id),
    created_at      TIMESTAMPTZ DEFAULT NOW(),
    updated_at      TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_taxonomy_parent ON taxonomy_tags(parent_id);
CREATE INDEX idx_taxonomy_status ON taxonomy_tags(status);
CREATE INDEX idx_taxonomy_type ON taxonomy_tags(tag_type);
CREATE INDEX idx_taxonomy_canonical ON taxonomy_tags(canonical_id);
CREATE INDEX idx_taxonomy_slug ON taxonomy_tags(slug);
CREATE INDEX idx_taxonomy_name_trgm ON taxonomy_tags USING gin (name gin_trgm_ops);
CREATE INDEX idx_taxonomy_pinyin_trgm ON taxonomy_tags USING gin (pinyin gin_trgm_ops);

CREATE TABLE taxonomy_synonyms (
    id              SERIAL PRIMARY KEY,
    term            VARCHAR(200) NOT NULL,
    canonical_tag_id INT NOT NULL REFERENCES taxonomy_tags(id) ON DELETE CASCADE,
    status          synonym_status DEFAULT 'active',
    created_by      INT REFERENCES users(id),
    reviewed_by     INT REFERENCES users(id),
    reviewed_at     TIMESTAMPTZ,
    created_at      TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(term, canonical_tag_id)
);

CREATE INDEX idx_synonyms_term ON taxonomy_synonyms(term);
CREATE INDEX idx_synonyms_term_trgm ON taxonomy_synonyms USING gin (term gin_trgm_ops);
CREATE INDEX idx_synonyms_canonical ON taxonomy_synonyms(canonical_tag_id);
CREATE INDEX idx_synonyms_status ON taxonomy_synonyms(status);

-- Taxonomy review queue for admin approval
CREATE TABLE taxonomy_review_queue (
    id              SERIAL PRIMARY KEY,
    entity_type     VARCHAR(20) NOT NULL,           -- 'tag' or 'synonym'
    entity_id       INT NOT NULL,
    action          taxonomy_review_action,
    reason          TEXT,
    submitted_by    INT REFERENCES users(id),
    reviewed_by     INT REFERENCES users(id),
    reviewed_at     TIMESTAMPTZ,
    decision_notes  TEXT,
    status          VARCHAR(20) DEFAULT 'pending',
    created_at      TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_review_queue_status ON taxonomy_review_queue(status);
CREATE INDEX idx_review_queue_entity ON taxonomy_review_queue(entity_type, entity_id);

-- ============================================================
-- Synonym Conflict Detection Trigger
-- Blocks two active synonyms from pointing to different canonical tags
-- ============================================================

CREATE OR REPLACE FUNCTION check_synonym_conflict()
RETURNS TRIGGER AS $$
DECLARE
    existing_canonical INT;
BEGIN
    IF NEW.status = 'active' THEN
        SELECT canonical_tag_id INTO existing_canonical
        FROM taxonomy_synonyms
        WHERE term = NEW.term
          AND status = 'active'
          AND canonical_tag_id != NEW.canonical_tag_id
          AND id != COALESCE(NEW.id, 0)
        LIMIT 1;

        IF existing_canonical IS NOT NULL THEN
            RAISE EXCEPTION
                'Synonym conflict: term "%" already has an active mapping to canonical tag ID %. '
                'Deactivate the existing synonym first or use the same canonical tag.',
                NEW.term, existing_canonical;
        END IF;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_synonym_conflict_check
    BEFORE INSERT OR UPDATE ON taxonomy_synonyms
    FOR EACH ROW
    EXECUTE FUNCTION check_synonym_conflict();

-- ============================================================
-- RESOURCES (Learning Library)
-- ============================================================

CREATE TYPE resource_status AS ENUM ('draft', 'published', 'archived');
CREATE TYPE resource_type AS ENUM ('article', 'video', 'course', 'document', 'link', 'assessment');

CREATE TABLE resources (
    id              SERIAL PRIMARY KEY,
    title           VARCHAR(500) NOT NULL,
    slug            VARCHAR(500) UNIQUE NOT NULL,
    description     TEXT,
    content_body    TEXT,
    resource_type   resource_type NOT NULL DEFAULT 'article',
    status          resource_status DEFAULT 'draft',
    category_id     INT REFERENCES taxonomy_tags(id),
    author_id       INT REFERENCES users(id),
    duration_mins   INT,
    difficulty      VARCHAR(20),                    -- beginner, intermediate, advanced
    thumbnail_url   TEXT,
    external_url    TEXT,
    view_count      INT DEFAULT 0,
    completion_count INT DEFAULT 0,
    popularity_score FLOAT DEFAULT 0,
    pinyin_title    VARCHAR(1000),                  -- pre-computed pinyin for title
    content_hash    VARCHAR(64),                    -- SHA-256 for near-duplicate detection
    search_vector   TSVECTOR,                       -- full-text search vector
    published_at    TIMESTAMPTZ,
    archived_at     TIMESTAMPTZ,
    created_at      TIMESTAMPTZ DEFAULT NOW(),
    updated_at      TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_resources_status ON resources(status);
CREATE INDEX idx_resources_category ON resources(category_id);
CREATE INDEX idx_resources_author ON resources(author_id);
CREATE INDEX idx_resources_published ON resources(published_at);
CREATE INDEX idx_resources_search ON resources USING gin(search_vector);
CREATE INDEX idx_resources_title_trgm ON resources USING gin(title gin_trgm_ops);
CREATE INDEX idx_resources_content_hash ON resources(content_hash);
CREATE INDEX idx_resources_popularity ON resources(popularity_score DESC);

-- Resource-Tag many-to-many
CREATE TABLE resource_tags (
    resource_id INT NOT NULL REFERENCES resources(id) ON DELETE CASCADE,
    tag_id      INT NOT NULL REFERENCES taxonomy_tags(id) ON DELETE CASCADE,
    PRIMARY KEY (resource_id, tag_id)
);

CREATE INDEX idx_resource_tags_tag ON resource_tags(tag_id);

-- ============================================================
-- Auto-update search_vector on resource insert/update
-- Combines title (weight A), description (weight B), content_body (weight C),
-- and pinyin_title (weight D) into tsvector
-- ============================================================

CREATE OR REPLACE FUNCTION resources_search_vector_update()
RETURNS TRIGGER AS $$
BEGIN
    NEW.search_vector :=
        setweight(to_tsvector('english', COALESCE(NEW.title, '')), 'A') ||
        setweight(to_tsvector('english', COALESCE(NEW.description, '')), 'B') ||
        setweight(to_tsvector('english', COALESCE(NEW.content_body, '')), 'C') ||
        setweight(to_tsvector('simple', COALESCE(NEW.pinyin_title, '')), 'D');
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_resources_search_vector
    BEFORE INSERT OR UPDATE OF title, description, content_body, pinyin_title
    ON resources
    FOR EACH ROW
    EXECUTE FUNCTION resources_search_vector_update();

-- ============================================================
-- Archive pages view (auto-generated by month and tag)
-- ============================================================

CREATE MATERIALIZED VIEW IF NOT EXISTS resource_archive_monthly AS
SELECT
    date_trunc('month', published_at) AS archive_month,
    category_id,
    COUNT(*) AS resource_count,
    array_agg(id ORDER BY published_at DESC) AS resource_ids
FROM resources
WHERE status = 'published' AND published_at IS NOT NULL
GROUP BY date_trunc('month', published_at), category_id
ORDER BY archive_month DESC;

CREATE MATERIALIZED VIEW IF NOT EXISTS resource_archive_by_tag AS
SELECT
    rt.tag_id,
    t.name AS tag_name,
    date_trunc('month', r.published_at) AS archive_month,
    COUNT(*) AS resource_count,
    array_agg(r.id ORDER BY r.published_at DESC) AS resource_ids
FROM resources r
JOIN resource_tags rt ON rt.resource_id = r.id
JOIN taxonomy_tags t ON t.id = rt.tag_id
WHERE r.status = 'published' AND r.published_at IS NOT NULL
GROUP BY rt.tag_id, t.name, date_trunc('month', r.published_at)
ORDER BY archive_month DESC;

-- ============================================================
-- LEARNING PATHS & PROGRESS
-- ============================================================

CREATE TYPE path_item_type AS ENUM ('required', 'elective');
CREATE TYPE progress_status AS ENUM ('not_started', 'in_progress', 'completed');
CREATE TYPE enrollment_status AS ENUM ('active', 'completed', 'dropped');

CREATE TABLE learning_paths (
    id                  SERIAL PRIMARY KEY,
    title               VARCHAR(500) NOT NULL,
    slug                VARCHAR(500) UNIQUE NOT NULL,
    description         TEXT,
    category_id         INT REFERENCES taxonomy_tags(id),
    target_job_family   VARCHAR(100),
    required_count      INT NOT NULL DEFAULT 0,     -- e.g., 6 required items
    elective_min        INT NOT NULL DEFAULT 0,     -- e.g., 2 electives minimum
    estimated_hours     FLOAT,
    difficulty          VARCHAR(20),
    is_active           BOOLEAN DEFAULT TRUE,
    created_by          INT REFERENCES users(id),
    created_at          TIMESTAMPTZ DEFAULT NOW(),
    updated_at          TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_paths_category ON learning_paths(category_id);
CREATE INDEX idx_paths_job_family ON learning_paths(target_job_family);

CREATE TABLE learning_path_items (
    id              SERIAL PRIMARY KEY,
    path_id         INT NOT NULL REFERENCES learning_paths(id) ON DELETE CASCADE,
    resource_id     INT NOT NULL REFERENCES resources(id) ON DELETE CASCADE,
    item_type       path_item_type NOT NULL DEFAULT 'required',
    sort_order      INT DEFAULT 0,
    created_at      TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(path_id, resource_id)
);

CREATE INDEX idx_path_items_path ON learning_path_items(path_id);
CREATE INDEX idx_path_items_resource ON learning_path_items(resource_id);

CREATE TABLE user_enrollments (
    id              SERIAL PRIMARY KEY,
    user_id         INT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    path_id         INT NOT NULL REFERENCES learning_paths(id) ON DELETE CASCADE,
    status          enrollment_status DEFAULT 'active',
    enrolled_at     TIMESTAMPTZ DEFAULT NOW(),
    completed_at    TIMESTAMPTZ,
    last_accessed   TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(user_id, path_id)
);

CREATE INDEX idx_enrollments_user ON user_enrollments(user_id);
CREATE INDEX idx_enrollments_path ON user_enrollments(path_id);

CREATE TABLE user_progress (
    id              SERIAL PRIMARY KEY,
    user_id         INT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    resource_id     INT NOT NULL REFERENCES resources(id) ON DELETE CASCADE,
    path_id         INT REFERENCES learning_paths(id) ON DELETE SET NULL,
    status          progress_status DEFAULT 'not_started',
    progress_pct    INT DEFAULT 0 CHECK (progress_pct BETWEEN 0 AND 100),
    time_spent_mins INT DEFAULT 0,
    last_position   TEXT,                           -- resume bookmark (e.g., timestamp, page)
    started_at      TIMESTAMPTZ,
    completed_at    TIMESTAMPTZ,
    synced_at       TIMESTAMPTZ DEFAULT NOW(),      -- cross-device sync timestamp
    created_at      TIMESTAMPTZ DEFAULT NOW(),
    updated_at      TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(user_id, resource_id, path_id)
);

CREATE INDEX idx_progress_user ON user_progress(user_id);
CREATE INDEX idx_progress_resource ON user_progress(resource_id);
CREATE INDEX idx_progress_path ON user_progress(path_id);
CREATE INDEX idx_progress_synced ON user_progress(synced_at);

-- Resource view/interaction events for recommendation engine
CREATE TABLE resource_events (
    id              BIGSERIAL PRIMARY KEY,
    user_id         INT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    resource_id     INT NOT NULL REFERENCES resources(id) ON DELETE CASCADE,
    event_type      VARCHAR(30) NOT NULL,           -- 'view', 'complete', 'bookmark', 'share'
    created_at      TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_events_user ON resource_events(user_id);
CREATE INDEX idx_events_resource ON resource_events(resource_id);
CREATE INDEX idx_events_type ON resource_events(event_type);

-- ============================================================
-- RECOMMENDATIONS (pre-computed by background worker)
-- ============================================================

CREATE TABLE recommendations (
    id              SERIAL PRIMARY KEY,
    user_id         INT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    resource_id     INT NOT NULL REFERENCES resources(id) ON DELETE CASCADE,
    score           FLOAT NOT NULL DEFAULT 0,
    reason          VARCHAR(100),                   -- 'similar_users', 'tag_match', 'job_family', 'popular', 'cold_start'
    category_id     INT REFERENCES taxonomy_tags(id),
    batch_id        VARCHAR(50),                    -- groups recs from same computation run
    created_at      TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(user_id, resource_id)
);

CREATE INDEX idx_recs_user ON recommendations(user_id);
CREATE INDEX idx_recs_score ON recommendations(user_id, score DESC);
CREATE INDEX idx_recs_category ON recommendations(category_id);
CREATE INDEX idx_recs_batch ON recommendations(batch_id);

-- ============================================================
-- PROCUREMENT: Vendors, Orders, Invoices
-- ============================================================

CREATE TYPE order_status AS ENUM (
    'draft', 'submitted', 'approved', 'rejected',
    'fulfilled', 'partially_fulfilled', 'cancelled'
);
CREATE TYPE invoice_status AS ENUM (
    'pending', 'matched', 'variance_detected', 'pending_approval',
    'manual_investigation', 'approved', 'rejected', 'paid'
);
CREATE TYPE ledger_entry_type AS ENUM ('AR', 'AP');
CREATE TYPE settlement_status AS ENUM (
    'open', 'matched', 'variance_pending', 'writeoff_suggested',
    'writeoff_approved', 'settled', 'disputed'
);

CREATE TABLE vendors (
    id              SERIAL PRIMARY KEY,
    name            VARCHAR(300) NOT NULL,
    code            VARCHAR(50) UNIQUE NOT NULL,
    contact_email   VARCHAR(255),
    contact_phone   VARCHAR(50),
    address         TEXT,
    is_active       BOOLEAN DEFAULT TRUE,
    created_at      TIMESTAMPTZ DEFAULT NOW(),
    updated_at      TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE procurement_orders (
    id              SERIAL PRIMARY KEY,
    order_number    VARCHAR(50) UNIQUE NOT NULL,
    vendor_id       INT NOT NULL REFERENCES vendors(id),
    status          order_status DEFAULT 'draft',
    department      VARCHAR(100),
    cost_center     VARCHAR(50),
    total_amount    NUMERIC(14,2) NOT NULL DEFAULT 0,
    currency        VARCHAR(3) DEFAULT 'USD',
    description     TEXT,
    submitted_by    INT REFERENCES users(id),
    approved_by     INT REFERENCES users(id),
    approved_at     TIMESTAMPTZ,
    created_at      TIMESTAMPTZ DEFAULT NOW(),
    updated_at      TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_orders_vendor ON procurement_orders(vendor_id);
CREATE INDEX idx_orders_status ON procurement_orders(status);
CREATE INDEX idx_orders_dept ON procurement_orders(department);
CREATE INDEX idx_orders_cost_center ON procurement_orders(cost_center);

CREATE TABLE order_line_items (
    id              SERIAL PRIMARY KEY,
    order_id        INT NOT NULL REFERENCES procurement_orders(id) ON DELETE CASCADE,
    description     VARCHAR(500) NOT NULL,
    quantity        NUMERIC(10,2) NOT NULL,
    unit_price      NUMERIC(14,2) NOT NULL,
    total_price     NUMERIC(14,2) NOT NULL,
    warehouse_code  VARCHAR(50),
    transport_code  VARCHAR(50),
    created_at      TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_line_items_order ON order_line_items(order_id);

CREATE TABLE invoices (
    id              SERIAL PRIMARY KEY,
    invoice_number  VARCHAR(50) UNIQUE NOT NULL,
    order_id        INT REFERENCES procurement_orders(id),
    vendor_id       INT NOT NULL REFERENCES vendors(id),
    status          invoice_status DEFAULT 'pending',
    invoice_amount  NUMERIC(14,2) NOT NULL,
    order_amount    NUMERIC(14,2),
    variance_amount NUMERIC(14,2),
    variance_pct    NUMERIC(5,2),
    currency        VARCHAR(3) DEFAULT 'USD',
    invoice_date    DATE NOT NULL,
    due_date        DATE,
    notes           TEXT,
    matched_by      INT REFERENCES users(id),
    matched_at      TIMESTAMPTZ,
    created_at      TIMESTAMPTZ DEFAULT NOW(),
    updated_at      TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_invoices_order ON invoices(order_id);
CREATE INDEX idx_invoices_vendor ON invoices(vendor_id);
CREATE INDEX idx_invoices_status ON invoices(status);

-- ============================================================
-- LEDGER: AR/AP Entries & Settlements
-- ============================================================

CREATE TABLE ledger_entries (
    id              SERIAL PRIMARY KEY,
    entry_type      ledger_entry_type NOT NULL,
    reference_type  VARCHAR(30) NOT NULL,
    reference_id    INT NOT NULL,
    vendor_id       INT NOT NULL REFERENCES vendors(id),
    amount          NUMERIC(14,2) NOT NULL,
    currency        VARCHAR(3) DEFAULT 'USD',
    department      VARCHAR(100),
    cost_center     VARCHAR(50),
    description     TEXT,
    posted_by       INT REFERENCES users(id),
    posted_at       TIMESTAMPTZ DEFAULT NOW(),
    created_at      TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_ledger_type ON ledger_entries(entry_type);
CREATE INDEX idx_ledger_vendor ON ledger_entries(vendor_id);
CREATE INDEX idx_ledger_ref ON ledger_entries(reference_type, reference_id);
CREATE INDEX idx_ledger_dept ON ledger_entries(department);
CREATE INDEX idx_ledger_cc ON ledger_entries(cost_center);

CREATE TABLE settlements (
    id              SERIAL PRIMARY KEY,
    vendor_id       INT NOT NULL REFERENCES vendors(id),
    status          settlement_status DEFAULT 'open',
    ar_total        NUMERIC(14,2) DEFAULT 0,
    ap_total        NUMERIC(14,2) DEFAULT 0,
    net_amount      NUMERIC(14,2) DEFAULT 0,
    variance_amount NUMERIC(14,2) DEFAULT 0,
    writeoff_amount NUMERIC(14,2) DEFAULT 0,
    department      VARCHAR(100),
    cost_center     VARCHAR(50),
    period_start    DATE,
    period_end      DATE,
    notes           TEXT,
    created_by      INT REFERENCES users(id),
    approved_by     INT REFERENCES users(id),
    approved_at     TIMESTAMPTZ,
    settled_at      TIMESTAMPTZ,
    created_at      TIMESTAMPTZ DEFAULT NOW(),
    updated_at      TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_settlements_vendor ON settlements(vendor_id);
CREATE INDEX idx_settlements_status ON settlements(status);

CREATE TABLE billing_rules (
    id              SERIAL PRIMARY KEY,
    rule_type       VARCHAR(30) NOT NULL,
    code            VARCHAR(50) NOT NULL,
    description     TEXT,
    rate_per_unit   NUMERIC(14,4) NOT NULL,
    min_charge      NUMERIC(14,2) DEFAULT 0,
    max_charge      NUMERIC(14,2),
    effective_from  DATE NOT NULL,
    effective_to    DATE,
    is_active       BOOLEAN DEFAULT TRUE,
    created_at      TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(rule_type, code, effective_from)
);

-- ============================================================
-- REVIEWS & DISPUTES
-- ============================================================

CREATE TYPE review_status AS ENUM ('visible', 'hidden', 'disclaimer');
CREATE TYPE dispute_status AS ENUM (
    'created', 'evidence_uploaded', 'under_review',
    'arbitration', 'resolved_hidden', 'resolved_disclaimer',
    'resolved_restored', 'rejected'
);

CREATE TABLE vendor_reviews (
    id              SERIAL PRIMARY KEY,
    vendor_id       INT NOT NULL REFERENCES vendors(id),
    order_id        INT REFERENCES procurement_orders(id),
    reviewer_id     INT NOT NULL REFERENCES users(id),
    rating          INT NOT NULL CHECK (rating BETWEEN 1 AND 5),
    title           VARCHAR(300),
    body            TEXT NOT NULL,
    image_urls      TEXT[],
    review_status   review_status DEFAULT 'visible',
    disclaimer_text TEXT,
    created_at      TIMESTAMPTZ DEFAULT NOW(),
    updated_at      TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_reviews_vendor ON vendor_reviews(vendor_id);
CREATE INDEX idx_reviews_status ON vendor_reviews(review_status);

CREATE TABLE merchant_replies (
    id              SERIAL PRIMARY KEY,
    review_id       INT NOT NULL REFERENCES vendor_reviews(id) ON DELETE CASCADE,
    vendor_id       INT NOT NULL REFERENCES vendors(id),
    body            TEXT NOT NULL,
    replied_by      INT REFERENCES users(id),
    created_at      TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE disputes (
    id                  SERIAL PRIMARY KEY,
    review_id           INT NOT NULL REFERENCES vendor_reviews(id),
    vendor_id           INT NOT NULL REFERENCES vendors(id),
    status              dispute_status DEFAULT 'created',
    reason              TEXT NOT NULL,
    evidence_urls       TEXT[],
    evidence_metadata_enc BYTEA,
    merchant_response   TEXT,
    arbitration_notes   TEXT,
    arbitration_outcome VARCHAR(30),
    arbitrated_by       INT REFERENCES users(id),
    arbitrated_at       TIMESTAMPTZ,
    created_by          INT NOT NULL REFERENCES users(id),
    created_at          TIMESTAMPTZ DEFAULT NOW(),
    updated_at          TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_disputes_review ON disputes(review_id);
CREATE INDEX idx_disputes_vendor ON disputes(vendor_id);
CREATE INDEX idx_disputes_status ON disputes(status);

-- Dispute State Machine Trigger
CREATE OR REPLACE FUNCTION validate_dispute_transition()
RETURNS TRIGGER AS $$
BEGIN
    IF TG_OP = 'UPDATE' AND OLD.status != NEW.status THEN
        IF NOT (
            (OLD.status = 'created'           AND NEW.status = 'evidence_uploaded') OR
            (OLD.status = 'evidence_uploaded'  AND NEW.status = 'under_review')     OR
            (OLD.status = 'under_review'       AND NEW.status = 'arbitration')      OR
            (OLD.status = 'arbitration'        AND NEW.status IN ('resolved_hidden', 'resolved_disclaimer', 'resolved_restored', 'rejected'))
        ) THEN
            RAISE EXCEPTION 'Invalid dispute transition from % to %', OLD.status, NEW.status;
        END IF;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_dispute_state_machine
    BEFORE UPDATE ON disputes
    FOR EACH ROW
    EXECUTE FUNCTION validate_dispute_transition();

-- Invoice Variance Auto-Classify Trigger
CREATE OR REPLACE FUNCTION auto_classify_invoice_variance()
RETURNS TRIGGER AS $$
DECLARE
    threshold NUMERIC(14,2);
BEGIN
    IF NEW.variance_amount IS NOT NULL AND NEW.status = 'variance_detected' THEN
        SELECT CAST(value AS NUMERIC) INTO threshold
        FROM configs WHERE key = 'variance.auto_writeoff_threshold';
        IF threshold IS NULL THEN threshold := 5.00; END IF;
        IF ABS(NEW.variance_amount) < threshold THEN
            NEW.status := 'pending_approval';
        ELSE
            NEW.status := 'manual_investigation';
        END IF;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_invoice_variance_classify
    BEFORE INSERT OR UPDATE OF variance_amount, status ON invoices
    FOR EACH ROW
    EXECUTE FUNCTION auto_classify_invoice_variance();

-- ============================================================
-- Auto-compute content_hash for near-duplicate detection
-- Uses SHA-256 of title + description + content_body
-- ============================================================

CREATE OR REPLACE FUNCTION compute_content_hash()
RETURNS TRIGGER AS $$
BEGIN
    NEW.content_hash := encode(
        digest(
            COALESCE(NEW.title, '') || '||' ||
            COALESCE(NEW.description, '') || '||' ||
            COALESCE(NEW.content_body, ''),
            'sha256'
        ),
        'hex'
    );
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_compute_content_hash
    BEFORE INSERT OR UPDATE OF title, description, content_body
    ON resources
    FOR EACH ROW
    EXECUTE FUNCTION compute_content_hash();

-- ============================================================
-- Scheduled Job Run History (for scheduler orchestrator)
-- ============================================================

CREATE TABLE scheduled_job_runs (
    id              BIGSERIAL PRIMARY KEY,
    job_id          INT NOT NULL REFERENCES scheduled_jobs(id) ON DELETE CASCADE,
    status          VARCHAR(20) NOT NULL DEFAULT 'running',  -- running, success, failed, compensated
    started_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    finished_at     TIMESTAMPTZ,
    error_message   TEXT,
    retry_attempt   INT DEFAULT 0,
    created_at      TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_job_runs_job ON scheduled_job_runs(job_id);
CREATE INDEX idx_job_runs_status ON scheduled_job_runs(status);

-- No seeded data: all content is created by users at runtime.

