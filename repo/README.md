# WLPR Portal - Workforce Learning & Procurement Reconciliation

A fully offline, on-premise enterprise portal for managing employee learning paths alongside vendor order governance and financial settlement. Built with Go (Echo), React (Vite), and PostgreSQL.

## Architecture Overview

```
┌─────────────┐     ┌──────────────────┐     ┌──────────────┐
│   React UI  │────▶│   Go/Echo API    │────▶│  PostgreSQL   │
│  (Nginx)    │     │   REST Backend   │     │   16          │
│  Port 3001  │     │   Port 8081      │     │   Port 5433   │
└─────────────┘     └──────────────────┘     └──────────────┘
                           │
                    ┌──────┴──────┐
                    │  Scheduler  │
                    │  + Workers  │
                    └─────────────┘
```

### Key Subsystems

- **Authentication & RBAC** - Local login with bcrypt hashing, optional TOTP MFA (AES-256 encrypted secrets), JWT tokens with role IDs, session management (15m idle / 8h max), 6 roles with 18 granular permissions
- **Offline Full-Text Search** - PostgreSQL `tsvector` with weighted fields (A=title, B=description, C=body, D=pinyin), `pg_trgm` for typo tolerance, synonym expansion, pinyin matching for Chinese characters, multi-dimensional filtering, near-duplicate deduplication via `content_hash`
- **Recommendation Engine** - Scheduler-driven nightly rebuild computing user-item cosine similarity, job-family cold-start defaults, content-hash deduplication, diversity control (max 40% from any single category)
- **Taxonomy** - Hierarchical job/skill tags with synonym conflict detection (DB trigger blocks two active synonyms pointing to different canonical tags), moderator review queue (submit/approve/reject workflow) with full audit trail
- **Learning Paths** - Required/elective completion rules (e.g., 6 required + 2 electives), per-resource progress tracking with resume bookmarks, cross-device sync, CSV export
- **Reconciliation State Machine** - Invoice matching with auto-variance classification ($5.00 threshold), settlement transitions (open -> matched -> settled), AR/AP ledger with cost allocation by department/cost center, file-drop export to CSV
- **Dispute Workflow** - Created -> Evidence Uploaded -> Under Review -> Arbitration -> Resolved. Arbitration outcomes control review visibility: hidden, shown with disclaimer, or restored. DB trigger enforces valid state transitions
- **Configuration Center** - Feature flags with rollout strategies (all, role-based with JWT role IDs, deterministic percentage bucketing, disabled), in-memory cache with 30s background sync, app version compatibility checks
- **Scheduled Jobs** - Centralized job orchestrator reading from `scheduled_jobs` table with persistent run history, retry with exponential backoff, and compensation handlers

## Access Points

| Service  | URL                        | Description                    |
|----------|----------------------------|--------------------------------|
| Web UI   | http://localhost:3001       | React frontend served by Nginx |
| API      | http://localhost:8081       | Go/Echo REST API               |
| Database | localhost:5433              | PostgreSQL 16                  |

## Getting Started: Registration

The system ships with **no seeded users** and **no seeded content**. All data is created by users at runtime.

1. **Register the first user** via the UI (`/register`) or API: `POST /api/auth/register`
   - The **first user** is automatically assigned the `system_admin` role with full access.
2. **Subsequent users** register normally and receive **no role** until an admin assigns one.
3. **Admin assigns roles** via the UI (Admin panel) or API: `POST /api/admin/users/assign-role`

### Available Roles

| Role                    | Access                                      |
|-------------------------|---------------------------------------------|
| `system_admin`          | Full access to all modules + admin panel     |
| `content_moderator`     | Learning library, taxonomy, content mgmt     |
| `learner`               | Learning paths, catalog, progress tracking   |
| `procurement_specialist`| Orders, reviews, disputes                    |
| `approver`              | Order/procurement approvals, reconciliation  |
| `finance_analyst`       | Reconciliation, settlements, cost allocation |

### RBAC Invisibility

Users without access to a feature **cannot discover it exists**:
- **Frontend**: Navigation menus and dashboard cards only show modules the user's role can access
- **Backend**: Unauthorized API requests return `404 Not Found` (not `403 Forbidden`)
- **Routes**: Frontend route guards render a generic 404 page for role-restricted pages

## Quick Start

### Docker (Recommended)

```bash
# Build and start all services
docker compose up --build -d

# Watch logs
docker compose logs -f

# Stop all services
docker compose down

# Full reset (removes DB data)
docker compose down -v
docker compose up --build -d
```

### Local Development (without Docker)

**Prerequisites:** Go 1.22+, Node 20+, PostgreSQL 16+

```bash
# 1. Database
createdb wlpr_portal
psql -d wlpr_portal -f backend/init.sql

# 2. Edit config/config.js with your local database credentials
#    Update DATABASE_URL to point to your local PostgreSQL.
#    For local dev, change the host from "db" to "localhost" and port to match your setup.

# 3. Backend
cd backend
go mod tidy
go run ./cmd/api
# The backend reads all config from ../config/config.js automatically.

# 4. Frontend (new terminal)
cd frontend
npm install
npm run dev
# Open http://localhost:5173
```

## Docker Commands Reference

```bash
# Build images only
docker compose build

# Start services in foreground
docker compose up

# Start services in background
docker compose up -d

# View logs (all services)
docker compose logs -f

# View logs (single service)
docker compose logs -f backend

# Stop services
docker compose down

# Stop and remove volumes (full data reset)
docker compose down -v

# Rebuild a single service
docker compose up --build backend

# Shell into a running container
docker exec -it wlpr-backend sh
docker exec -it wlpr-db psql -U wlpr -d wlpr_portal
```

## API Endpoints

### Authentication
| Method | Endpoint                    | Auth | Description              |
|--------|-----------------------------|------|--------------------------|
| POST   | `/api/auth/register`        | No   | Register new user        |
| POST   | `/api/auth/login`           | No   | Login with credentials   |
| POST   | `/api/auth/mfa/verify`      | No   | Verify TOTP code         |
| POST   | `/api/auth/logout`          | Yes  | Logout / revoke session  |
| GET    | `/api/auth/me`              | Yes  | Current user info        |
| POST   | `/api/auth/mfa/setup`       | Yes  | Generate TOTP secret     |
| POST   | `/api/auth/mfa/confirm`     | Yes  | Enable MFA               |
| POST   | `/api/auth/mfa/disable`     | Yes  | Disable MFA              |

### Search & Resources
| Method | Endpoint                    | Auth | Description                           |
|--------|-----------------------------|------|---------------------------------------|
| GET    | `/api/search?q=...`         | Yes  | Full-text search (deduped by content) |
| GET    | `/api/resources/:id`        | Yes  | Resource detail                       |
| GET    | `/api/archives`             | Yes  | Monthly archive pages                 |

### Taxonomy
| Method | Endpoint                              | Auth      | Description                     |
|--------|---------------------------------------|-----------|---------------------------------|
| GET    | `/api/taxonomy/tags`                  | Yes       | List active tags                |
| GET    | `/api/taxonomy/tags/hierarchy`        | Yes       | Tag tree (active only)          |
| GET    | `/api/taxonomy/tags/:id`              | Yes       | Tag detail with children        |
| POST   | `/api/taxonomy/tags`                  | Moderator | Submit new tag (pending review) |
| GET    | `/api/taxonomy/synonyms/:tag_id`      | Yes       | List synonyms for a tag         |
| POST   | `/api/taxonomy/synonyms`              | Moderator | Submit synonym (pending review) |
| GET    | `/api/taxonomy/review-queue`          | Moderator | Pending review items            |
| GET    | `/api/taxonomy/review-queue/audit`    | Moderator | Full review audit trail         |
| POST   | `/api/taxonomy/review-queue/approve`  | Moderator | Approve a pending item          |
| POST   | `/api/taxonomy/review-queue/reject`   | Moderator | Reject a pending item           |

### Learning
| Method | Endpoint                          | Auth | Description                |
|--------|-----------------------------------|------|----------------------------|
| GET    | `/api/learning/paths`             | Yes  | List learning paths        |
| GET    | `/api/learning/paths/:id`         | Yes  | Path detail with items     |
| POST   | `/api/learning/enroll`            | Yes  | Enroll in path             |
| DELETE | `/api/learning/enroll/:path_id`   | Yes  | Drop enrollment            |
| GET    | `/api/learning/enrollments`       | Yes  | User's enrollments         |
| GET    | `/api/learning/enrollments/:id`   | Yes  | Enrollment detail          |
| PUT    | `/api/learning/progress`          | Yes  | Update progress            |
| GET    | `/api/learning/progress`          | Yes  | User progress records      |
| GET    | `/api/learning/export`            | Yes  | Download CSV records       |
| GET    | `/api/learning/recommendations`   | Yes  | Personalized recs (deduped)|

### Procurement & Finance
| Method | Endpoint                                  | Auth      | Description                |
|--------|-------------------------------------------|-----------|----------------------------|
| GET    | `/api/procurement/vendors`                | Proc+     | List vendors               |
| GET    | `/api/procurement/orders`                 | Proc+     | List orders                |
| GET    | `/api/procurement/orders/:id`             | Proc+     | Order detail               |
| PUT    | `/api/procurement/orders/:id/approve`     | Approver  | Approve order              |
| GET    | `/api/procurement/invoices`               | Proc+     | List invoices              |
| GET    | `/api/procurement/invoices/:id`           | Proc+     | Invoice detail             |
| POST   | `/api/procurement/invoices/match`         | Approver  | Match invoice to order     |
| GET    | `/api/procurement/reviews`                | Proc      | List vendor reviews        |
| POST   | `/api/procurement/reviews`                | Proc      | Create review              |
| POST   | `/api/procurement/reviews/reply`          | Proc      | Merchant reply             |
| GET    | `/api/procurement/disputes`               | Proc+     | List disputes              |
| GET    | `/api/procurement/disputes/:id`           | Proc+     | Dispute detail             |
| POST   | `/api/procurement/disputes`               | Proc      | Create dispute             |
| POST   | `/api/procurement/disputes/transition`    | Moderator | Dispute state change       |
| POST   | `/api/procurement/reconciliation/compare` | Finance   | Compare statement          |
| GET    | `/api/procurement/ledger`                 | Finance   | List ledger entries        |
| POST   | `/api/procurement/ledger`                 | Finance   | Create AR/AP entry         |
| GET    | `/api/procurement/settlements`            | Finance   | List settlements           |
| POST   | `/api/procurement/settlements`            | Finance   | Create settlement          |
| POST   | `/api/procurement/settlements/transition` | Finance   | Settlement state change    |
| GET    | `/api/procurement/cost-allocation`        | Finance   | Cost allocation report     |
| GET    | `/api/procurement/billing-rules`          | Proc+     | List billing rules         |
| GET    | `/api/procurement/export/ledger`          | Finance   | Export ledger to CSV       |
| GET    | `/api/procurement/export/settlements`     | Finance   | Export settlements to CSV  |

### Admin & Config
| Method | Endpoint                    | Auth  | Description                        |
|--------|-----------------------------|-------|------------------------------------|
| GET    | `/api/admin/users`          | Admin | List all users                     |
| POST   | `/api/admin/users/assign-role`| Admin | Assign role to user              |
| GET    | `/api/config/all`           | Admin | All system configs                 |
| GET    | `/api/config/:key`          | Admin | Get config value                   |
| PUT    | `/api/config/:key`          | Admin | Update config value                |
| GET    | `/api/config/flags`         | Admin | All feature flags                  |
| GET    | `/api/config/flags/:key`    | Admin | Get feature flag                   |
| PUT    | `/api/config/flags/:key`    | Admin | Update feature flag                |
| GET    | `/api/flags/:key/check`     | Yes   | Check flag for current user        |

## Project Structure

```
├── docker-compose.yml
├── run_test.sh / run_test.ps1            # Full test suite runners
├── config/
│   └── config.js                         # Single source of truth for all configuration
├── backend/
│   ├── Dockerfile
│   ├── go.mod
│   ├── init.sql                          # Full DB schema + seed data + triggers
│   ├── cmd/api/main.go                   # Application entry point
│   ├── internal/
│   │   ├── handlers/                     # HTTP route handlers
│   │   │   ├── auth.go, config.go, search.go
│   │   │   ├── taxonomy.go, learning.go, procurement.go
│   │   ├── middleware/                   # Auth, RBAC, logging, version check
│   │   ├── models/                       # Domain structs & request/response types
│   │   ├── repository/                   # PostgreSQL data access layer
│   │   │   ├── user_repo.go, config_repo.go, search_repo.go
│   │   │   ├── taxonomy_repo.go, learning_repo.go, procurement_repo.go
│   │   │   └── scheduler_repo.go        # Scheduled job run persistence
│   │   └── services/                     # Business logic & background workers
│   │       ├── auth_service.go, config_service.go, mfa_service.go
│   │       ├── search_service.go, learning_service.go
│   │       ├── reconciliation.go         # State machines & variance logic
│   │       ├── recommendation_worker.go  # Similarity engine with dedup
│   │       ├── scheduler.go             # Centralized job orchestrator
│   │       └── export_sink.go           # File-drop & webhook export
│   └── pkg/
│       ├── config/                       # config.js parser (single config source)
│       ├── crypto/                       # AES-256-GCM & bcrypt
│       ├── jwt/                          # Token issuance & validation (with role IDs)
│       └── pinyin/                       # Chinese character -> pinyin conversion
├── frontend/
│   ├── Dockerfile
│   ├── nginx.conf                        # Nginx with React Router push-state
│   ├── src/
│   │   ├── api/client.js                 # Axios with auth/version interceptors
│   │   ├── store/                        # Zustand state (auth, config)
│   │   ├── components/                   # Reusable UI components
│   │   └── pages/                        # Route-specific views
│   │       ├── Login.jsx, Dashboard.jsx, AdminConfig.jsx
│   │       ├── Catalog.jsx, LearningDashboard.jsx
│   │       ├── Reconciliation.jsx, DisputeQueue.jsx
```

## Configuration

All configuration is managed through a single file: `config/config.js`. There are no `.env` files. Both the frontend and backend read from this one file.

| File                  | Description                                              |
|-----------------------|----------------------------------------------------------|
| `config/config.js`    | Single source of truth for all frontend + backend config |

### Configuration values in `config/config.js`:

| Key                  | Default                                    | Used by  | Description                          |
|----------------------|--------------------------------------------|----------|--------------------------------------|
| `DATABASE_URL`       | `postgres://wlpr:wlpr_secret@db:5432/...`  | Backend  | PostgreSQL connection string         |
| `JWT_SECRET`         | *(required, min 32 chars)*                 | Backend  | HMAC signing key for JWT tokens      |
| `AES_ENCRYPTION_KEY` | *(required, 64 hex chars = 32 bytes)*      | Backend  | AES-256 key for MFA secret encryption|
| `PORT`               | `8080`                                     | Backend  | Backend internal listen port         |
| `POSTGRES_DB`        | `wlpr_portal`                              | Database | PostgreSQL database name             |
| `POSTGRES_USER`      | `wlpr`                                     | Database | PostgreSQL username                  |
| `POSTGRES_PASSWORD`  | `wlpr_secret`                              | Database | PostgreSQL password                  |
| `API_BASE_URL`       | `/api`                                     | Frontend | Frontend API base URL                |
| `APP_VERSION`        | `1.0.0`                                    | Frontend | Client version for compatibility     |

The frontend loads `config.js` via `<script src="/config.js">` before the React app boots. The backend parses the same file at startup to extract database, JWT, and AES settings. Override at deploy time by mounting a replacement via Docker volume.

## Running Tests

### Non-Docker (Local Go Toolchain)

**Prerequisites:** Go 1.22+

Unit tests run without any database or Docker dependency:

```bash
cd backend

# Run all unit tests (no DB required)
go test ./internal/services/... ./internal/handlers/... ./internal/middleware/... ./pkg/...

# Run with verbose output
go test -v ./internal/services/... ./internal/handlers/... ./internal/middleware/... ./pkg/...

# Run specific test suites
go test -v ./internal/services/ -run TestIsFlagEnabled      # Feature flag rollout tests
go test -v ./internal/services/ -run TestDeduplicate         # Near-duplicate dedup tests
go test -v ./internal/services/ -run TestReviewQueue         # Taxonomy review queue tests
go test -v ./internal/services/ -run TestParseCronInterval   # Scheduler cron parsing tests
go test -v ./internal/services/ -run TestSynonymConflict     # Synonym conflict detection tests
go test -v ./internal/handlers/ -run TestAPI_RBAC            # RBAC enforcement tests
go test -v ./internal/handlers/ -run TestAPI_CrossTenant     # Data isolation tests
go test -v ./internal/middleware/ -run TestAppVersionCheck   # Version gate tests
go test -v ./pkg/crypto/...                                  # AES & bcrypt tests
go test -v ./pkg/config/...                                  # config.js parser tests
```

Integration tests (require PostgreSQL):

```bash
# Start a local PostgreSQL and run init.sql first
createdb wlpr_portal_test
psql -d wlpr_portal_test -f backend/init.sql

export TEST_DATABASE_URL="postgres://your_user:your_pass@localhost:5432/wlpr_portal_test?sslmode=disable"
go test -v -tags=integration ./internal/repository/...
```

### Frontend Tests

```bash
cd frontend
npm install
npm run test     # Vitest unit tests (23 tests)
npm run build    # Verify production build compiles
```

### Docker (Full Suite)

```bash
# Full test suite (builds Docker, runs all tests, cleans up)
./run_test.sh          # macOS/Linux
./run_test.ps1         # Windows PowerShell
```

The test runner executes:
1. Docker build (validates compilation)
2. Service health checks (backend API + frontend + config.js)
3. API smoke tests (login, search, learning, procurement, RBAC)
4. Integration tests against containerized PostgreSQL (dispute state machine, FTS triggers, synonym conflicts, dedup, taxonomy review queue, scheduler run lifecycle)
5. Unit tests (AES crypto, config parser, variance logic, taxonomy conflicts, API security, feature flag rollout, recommendation dedup)
6. Frontend tests (Vitest)
7. Cleanup (docker compose down -v)

## Security Notes

- Passwords are hashed with bcrypt (cost 12). Accounts lock after 5 failed attempts for 15 minutes.
- TOTP MFA secrets are encrypted at rest using AES-256-GCM before storage in PostgreSQL.
- Sessions enforce both idle timeout (15 min default) and max lifetime (8 hours default), configurable via the admin panel.
- All API routes except login/register/MFA verify require a valid JWT with session validation.
- JWT tokens include role IDs for deterministic feature flag evaluation (percentage-based rollout uses FNV32a hash bucketing).
- Role-based access control is enforced at the Echo middleware level — handlers never execute without passing RBAC checks.
- Taxonomy changes (tags, synonyms) go through a moderator review queue before becoming active. All submissions and review decisions are recorded in the audit log.
- Dispute evidence metadata is encrypted at rest. Review content is masked in non-admin views based on arbitration outcomes.
- Near-duplicate resources are automatically detected via SHA-256 content hashing and collapsed in search results and recommendations.
- The `AES_ENCRYPTION_KEY` and `JWT_SECRET` values in `config/config.js` are for development only. Replace them in production.
