#!/usr/bin/env pwsh
$ErrorActionPreference = "Continue"
$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
Set-Location $scriptDir

$failed = 0
function Log-Pass($msg) { Write-Host "[PASS] $msg" -ForegroundColor Green }
function Log-Fail($msg) { Write-Host "[FAIL] $msg" -ForegroundColor Red; $script:failed = 1 }
function Log-Info($msg) { Write-Host "[INFO] $msg" -ForegroundColor Yellow }

# Port configuration (must match docker-compose.yml)
$API_PORT = 8081
$UI_PORT = 3001
$APP_VERSION = "1.0.0"

Log-Info "Phase 1: Building and starting Docker containers..."
docker compose down -v --remove-orphans 2>$null
$buildResult = docker compose --progress=plain build --no-cache 2>&1
if ($LASTEXITCODE -eq 0) { Log-Pass "Docker build succeeded" }
else { Log-Fail "Docker build failed"; Write-Host $buildResult; exit 1 }

# Start containers (may exit non-zero due to health check race; we poll manually)
docker compose up -d 2>$null
Log-Info "Waiting for backend to become healthy..."
$healthy = $false
for ($i = 1; $i -le 30; $i++) {
    try {
        $resp = Invoke-RestMethod -Uri "http://localhost:${API_PORT}/api/health" -TimeoutSec 3 -ErrorAction SilentlyContinue
        if ($resp.status -eq "ok") { $healthy = $true; break }
    } catch {}
    Start-Sleep -Seconds 2
}
if ($healthy) { Log-Pass "Backend is healthy" }
else { Log-Fail "Backend not healthy"; docker compose logs; docker compose down -v; exit 1 }

Log-Info "Waiting for frontend to become healthy..."
$feHealthy = $false
for ($i = 1; $i -le 30; $i++) {
    try {
        $null = Invoke-WebRequest -Uri "http://localhost:${UI_PORT}/" -TimeoutSec 3 -ErrorAction SilentlyContinue
        $feHealthy = $true; break
    } catch {}
    Start-Sleep -Seconds 2
}
if ($feHealthy) { Log-Pass "Frontend is healthy" }
else { Log-Fail "Frontend not healthy"; docker compose logs frontend; docker compose down -v; exit 1 }

Log-Info "Phase 2: Verifying endpoints..."
try { $h = Invoke-RestMethod -Uri "http://localhost:${API_PORT}/api/health"; if ($h.status -eq "ok") { Log-Pass "Backend health OK" } } catch { Log-Fail "Backend health failed" }
try { $f = Invoke-WebRequest -Uri "http://localhost:${UI_PORT}/"; if ($f.Content -match "WLPR Portal") { Log-Pass "Frontend OK" } } catch { Log-Fail "Frontend failed" }
try { $c = Invoke-WebRequest -Uri "http://localhost:${UI_PORT}/config.js"; if ($c.Content -match "__WLPR_CONFIG__") { Log-Pass "config.js OK" } } catch { Log-Fail "config.js failed" }

Log-Info "Phase 3: Running API smoke tests..."
$net = (docker network ls --format '{{.Name}}' | Select-String 'repo' | Select-Object -First 1).ToString().Trim()
docker run --rm --network=$net -e "TEST_API_URL=http://backend:8080" -v "${scriptDir}:/src:ro" golang:1.22-alpine sh -c "cp -r /src /app && cd /app && if [ ! -f go.work ]; then go work init && go work use ./backend ./tests; fi && cd tests && go mod tidy && cd .. && go test -v -tags=integration -count=1 -timeout=60s ./tests/e2e/backend/smoke/... 2>&1"
if ($LASTEXITCODE -eq 0) { Log-Pass "API smoke tests passed" } else { Log-Fail "API smoke tests failed" }

Log-Info "Phase 4: Integration tests..."
docker run --rm --network=$net -e "TEST_DATABASE_URL=postgres://wlpr:wlpr_secret@db:5432/wlpr_portal?sslmode=disable" -v "${scriptDir}:/src:ro" golang:1.22-alpine sh -c "cp -r /src /app && cd /app && if [ ! -f go.work ]; then go work init && go work use ./backend ./tests; fi && cd tests && go mod tidy && cd .. && go test -v -tags=integration -count=1 -timeout=120s ./tests/e2e/backend/repository/... 2>&1"
if ($LASTEXITCODE -eq 0) { Log-Pass "Integration tests passed" } else { Log-Fail "Integration tests failed" }

Log-Info "Phase 5: Unit tests..."
docker run --rm -v "${scriptDir}:/src:ro" golang:1.22-alpine sh -c "cp -r /src /app && cd /app && if [ ! -f go.work ]; then go work init && go work use ./backend ./tests; fi && cd tests && go mod tidy && cd .. && go test -v -count=1 -timeout=120s ./tests/unit/backend/... ./tests/api/backend/... 2>&1"
if ($LASTEXITCODE -eq 0) { Log-Pass "Backend unit and API tests passed" } else { Log-Fail "Backend unit and API tests failed" }

Log-Info "Phase 6: Frontend tests..."
docker run --rm -v "${scriptDir}:/src:ro" node:20-alpine sh -c "cp -r /src /app && cd /app/frontend && npm install --legacy-peer-deps 2>&1 | tail -1 && ln -sfn /app/frontend/node_modules /app/tests/unit/frontend/node_modules && npx vitest run 2>&1"
if ($LASTEXITCODE -eq 0) { Log-Pass "Frontend tests passed" } else { Log-Fail "Frontend tests failed" }

Log-Info "Phase 7: Cleanup..."
docker compose down -v --remove-orphans

if ($failed -eq 0) { Write-Host "`n  ALL TESTS PASSED" -ForegroundColor Green; exit 0 }
else { Write-Host "`n  SOME TESTS FAILED" -ForegroundColor Red; exit 1 }
