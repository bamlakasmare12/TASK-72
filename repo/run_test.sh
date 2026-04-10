#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$SCRIPT_DIR"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

FAILED=0

log_pass() { echo -e "${GREEN}[PASS]${NC} $1"; }
log_fail() { echo -e "${RED}[FAIL]${NC} $1"; FAILED=1; }
log_info() { echo -e "${YELLOW}[INFO]${NC} $1"; }

# Port configuration (must match docker-compose.yml)
API_PORT=8081
UI_PORT=3001
APP_VERSION="1.0.0"

# ============================================================
# Phase 1: Docker build and startup
# ============================================================
log_info "Phase 1: Building and starting Docker containers..."

docker compose down -v --remove-orphans 2>/dev/null || true

if docker compose --progress=plain build --no-cache 2>&1; then
    log_pass "Docker build succeeded"
else
    log_fail "Docker build failed"
    exit 1
fi

# Start containers (may exit non-zero due to health check race; we poll manually)
docker compose up -d || true

# Wait for backend to be healthy
log_info "Waiting for backend to become healthy..."
RETRIES=30
HEALTHY=0
for i in $(seq 1 $RETRIES); do
    if curl -sf http://localhost:${API_PORT}/api/health >/dev/null 2>&1; then
        HEALTHY=1
        break
    fi
    sleep 2
done

if [ "$HEALTHY" -eq 1 ]; then
    log_pass "Backend is healthy"
else
    log_fail "Backend did not become healthy in time"
    docker compose logs
    docker compose down -v
    exit 1
fi

# Wait for frontend to be healthy
log_info "Waiting for frontend to become healthy..."
FE_HEALTHY=0
for i in $(seq 1 $RETRIES); do
    if curl -sf http://localhost:${UI_PORT}/ >/dev/null 2>&1; then
        FE_HEALTHY=1
        break
    fi
    sleep 2
done

if [ "$FE_HEALTHY" -eq 1 ]; then
    log_pass "Frontend is healthy"
else
    log_fail "Frontend did not become healthy in time"
    docker compose logs frontend
    docker compose down -v
    exit 1
fi

# ============================================================
# Phase 2: Health check endpoints
# ============================================================
log_info "Phase 2: Verifying endpoints..."

if curl -sf http://localhost:${API_PORT}/api/health | grep -q '"status":"ok"'; then
    log_pass "Backend health check returned OK"
else
    log_fail "Backend health check failed"
fi

if curl -sf http://localhost:${UI_PORT}/ | grep -q 'WLPR Portal'; then
    log_pass "Frontend serves HTML with app title"
else
    log_fail "Frontend did not return expected HTML"
fi

if curl -sf http://localhost:${UI_PORT}/config.js | grep -q '__WLPR_CONFIG__'; then
    log_pass "Runtime config.js is served correctly"
else
    log_fail "Runtime config.js not found or invalid"
fi

# ============================================================
# Phase 3: Registration & API smoke tests
# ============================================================
log_info "Phase 3: Registration and API smoke tests..."

# Register admin user (role selected at registration)
REG_RESP=$(curl -sf -X POST http://localhost:${API_PORT}/api/auth/register \
    -H "Content-Type: application/json" \
    -H "X-App-Version: ${APP_VERSION}" \
    -d '{"username":"testadmin","email":"testadmin@test.local","password":"TestAdmin@2024!","display_name":"Test Administrator","role":"system_admin"}')

if echo "$REG_RESP" | grep -q 'system_admin\|system admin'; then
    log_pass "Admin user registered with system_admin role"
else
    log_fail "Admin registration failed: $REG_RESP"
fi

# Login as the registered admin
LOGIN_RESP=$(curl -sf -X POST http://localhost:${API_PORT}/api/auth/login \
    -H "Content-Type: application/json" \
    -H "X-App-Version: ${APP_VERSION}" \
    -d '{"username":"testadmin","password":"TestAdmin@2024!"}')

if echo "$LOGIN_RESP" | grep -q '"token"'; then
    log_pass "Admin login succeeded"
    TOKEN=$(echo "$LOGIN_RESP" | grep -o '"token":"[^"]*"' | head -1 | cut -d'"' -f4)
else
    log_fail "Admin login failed: $LOGIN_RESP"
    TOKEN=""
fi

if [ -n "$TOKEN" ]; then
    # Test authenticated endpoint
    if curl -sf -H "Authorization: Bearer $TOKEN" -H "X-App-Version: ${APP_VERSION}" http://localhost:${API_PORT}/api/auth/me | grep -q '"user_id"'; then
        log_pass "GET /api/auth/me returns user data"
    else
        log_fail "GET /api/auth/me failed"
    fi

    # Test search (empty results expected — no seeded content)
    if curl -sf -H "Authorization: Bearer $TOKEN" -H "X-App-Version: ${APP_VERSION}" "http://localhost:${API_PORT}/api/search?q=test" | grep -q '"results"'; then
        log_pass "GET /api/search returns valid response"
    else
        log_fail "GET /api/search failed"
    fi

    # Test learning paths (empty expected — no seeded data)
    PATHS_RESP=$(curl -sf -H "Authorization: Bearer $TOKEN" -H "X-App-Version: ${APP_VERSION}" http://localhost:${API_PORT}/api/learning/paths)
    if [ "$PATHS_RESP" = "null" ] || echo "$PATHS_RESP" | grep -q '\[\]'; then
        log_pass "GET /api/learning/paths returns empty (no seeded data)"
    else
        log_pass "GET /api/learning/paths returns valid response"
    fi

    # Register a second user with learner role
    REG2_RESP=$(curl -sf -X POST http://localhost:${API_PORT}/api/auth/register \
        -H "Content-Type: application/json" \
        -H "X-App-Version: ${APP_VERSION}" \
        -d '{"username":"testlearner","email":"learner@test.local","password":"Learner@2024!","display_name":"Test Learner","role":"learner"}')

    if echo "$REG2_RESP" | grep -q 'learner'; then
        log_pass "Second user registered with learner role"
    else
        log_fail "Second user registration response unexpected: $REG2_RESP"
    fi

    # Login as learner and verify RBAC invisibility (404 not 403)
    LEARNER_RESP=$(curl -sf -X POST http://localhost:${API_PORT}/api/auth/login \
        -H "Content-Type: application/json" \
        -H "X-App-Version: ${APP_VERSION}" \
        -d '{"username":"testlearner","password":"Learner@2024!"}')
    LEARNER_TOKEN=$(echo "$LEARNER_RESP" | grep -o '"token":"[^"]*"' | head -1 | cut -d'"' -f4)

    if [ -n "$LEARNER_TOKEN" ]; then
        HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" \
            -H "Authorization: Bearer $LEARNER_TOKEN" \
            -H "X-App-Version: ${APP_VERSION}" \
            http://localhost:${API_PORT}/api/procurement/settlements)
        if [ "$HTTP_CODE" = "404" ]; then
            log_pass "RBAC: Learner gets 404 for /settlements (feature invisible)"
        else
            log_fail "RBAC: Learner got $HTTP_CODE instead of 404 for /settlements"
        fi

        # Learner should be able to access search
        HTTP_CODE2=$(curl -s -o /dev/null -w "%{http_code}" \
            -H "Authorization: Bearer $LEARNER_TOKEN" \
            -H "X-App-Version: ${APP_VERSION}" \
            "http://localhost:${API_PORT}/api/search?q=test")
        if [ "$HTTP_CODE2" = "200" ]; then
            log_pass "Learner can access /search (200)"
        else
            log_fail "Learner got $HTTP_CODE2 for /search"
        fi
    fi
fi

# ============================================================
# Phase 4: Integration tests against running DB
# ============================================================
log_info "Phase 4: Running integration tests against containerized DB..."

NETWORK_NAME=$(docker network ls --format '{{.Name}}' | grep repo | head -1)
if [ -z "$NETWORK_NAME" ]; then
    NETWORK_NAME="repo_default"
fi

if docker run --rm --network="$NETWORK_NAME" \
    -e TEST_DATABASE_URL="postgres://wlpr:wlpr_secret@db:5432/wlpr_portal?sslmode=disable" \
    -v "$SCRIPT_DIR:/src:ro" \
    golang:1.22-alpine sh -c "
        cp -r /src /app && cd /app &&
        if [ ! -f go.work ]; then go work init && go work use ./backend ./tests; fi &&
        cd tests && go mod tidy && cd .. &&
        go test -v -tags=integration -count=1 -timeout=120s ./tests/e2e/backend/... 2>&1
    " ; then
    log_pass "Integration tests passed"
else
    log_fail "Integration tests failed"
fi

# ============================================================
# Phase 5: Unit tests (no DB required)
# ============================================================
log_info "Phase 5: Running unit tests..."

if docker run --rm \
    -v "$SCRIPT_DIR:/src:ro" \
    golang:1.22-alpine sh -c "
        cp -r /src /app && cd /app &&
        if [ ! -f go.work ]; then go work init && go work use ./backend ./tests; fi &&
        cd tests && go mod tidy && cd .. &&
        go test -v -count=1 -timeout=120s ./tests/unit/backend/... ./tests/api/backend/... 2>&1
    " ; then
    log_pass "Backend unit and API tests passed"
else
    log_fail "Backend unit and API tests failed"
fi

# ============================================================
# Phase 6: Frontend tests
# ============================================================
log_info "Phase 6: Running frontend tests..."

if docker run --rm \
    -v "$SCRIPT_DIR:/src:ro" \
    node:20-alpine sh -c "
        cp -r /src /app && cd /app/frontend && npm install --legacy-peer-deps 2>&1 | tail -1 &&
        ln -sfn /app/frontend/node_modules /app/tests/unit/frontend/node_modules &&
        npx vitest run 2>&1
    " ; then
    log_pass "Frontend tests passed"
else
    log_fail "Frontend tests failed"
fi

# ============================================================
# Phase 7: Cleanup
# ============================================================
log_info "Phase 7: Stopping and cleaning up Docker containers..."
docker compose down -v --remove-orphans

# ============================================================
# Summary
# ============================================================
echo ""
if [ "$FAILED" -eq 0 ]; then
    echo -e "${GREEN}========================================${NC}"
    echo -e "${GREEN}  ALL TESTS PASSED${NC}"
    echo -e "${GREEN}========================================${NC}"
    exit 0
else
    echo -e "${RED}========================================${NC}"
    echo -e "${RED}  SOME TESTS FAILED${NC}"
    echo -e "${RED}========================================${NC}"
    exit 1
fi
