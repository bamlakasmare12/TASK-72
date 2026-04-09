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

Log-Info "Phase 1: Building and starting Docker containers..."
docker compose down -v --remove-orphans 2>$null
$buildResult = docker compose build --no-cache 2>&1
if ($LASTEXITCODE -eq 0) { Log-Pass "Docker build succeeded" }
else { Log-Fail "Docker build failed"; Write-Host $buildResult; exit 1 }

docker compose up -d
Log-Info "Waiting for services to become healthy..."
$healthy = $false
for ($i = 1; $i -le 30; $i++) {
    try {
        $resp = Invoke-RestMethod -Uri "http://localhost:${API_PORT}/api/health" -TimeoutSec 3 -ErrorAction SilentlyContinue
        if ($resp.status -eq "ok") { $healthy = $true; break }
    } catch {}
    Start-Sleep -Seconds 2
}
if ($healthy) { Log-Pass "All services are healthy" }
else { Log-Fail "Services not healthy"; docker compose logs; docker compose down -v; exit 1 }

Log-Info "Phase 2: Verifying endpoints..."
try { $h = Invoke-RestMethod -Uri "http://localhost:${API_PORT}/api/health"; if ($h.status -eq "ok") { Log-Pass "Backend health OK" } } catch { Log-Fail "Backend health failed" }
try { $f = Invoke-WebRequest -Uri "http://localhost:${UI_PORT}/"; if ($f.Content -match "WLPR Portal") { Log-Pass "Frontend OK" } } catch { Log-Fail "Frontend failed" }
try { $c = Invoke-WebRequest -Uri "http://localhost:${UI_PORT}/config.js"; if ($c.Content -match "__WLPR_CONFIG__") { Log-Pass "config.js OK" } } catch { Log-Fail "config.js failed" }

Log-Info "Phase 3: Registration and API smoke tests..."
try {
    $reg = Invoke-RestMethod -Method Post -Uri "http://localhost:${API_PORT}/api/auth/register" -ContentType "application/json" -Body '{"username":"testadmin","email":"ta@test.local","password":"TestAdmin@2024!","display_name":"Test Admin","role":"system_admin"}'
    if ($reg.message) { Log-Pass "First user registered (auto-admin)" } else { Log-Fail "Registration failed" }
} catch { Log-Fail "Registration error: $_" }

try {
    $login = Invoke-RestMethod -Method Post -Uri "http://localhost:${API_PORT}/api/auth/login" -ContentType "application/json" -Body '{"username":"testadmin","password":"TestAdmin@2024!"}'
    if ($login.token) {
        Log-Pass "Admin login succeeded"
        $headers = @{ "Authorization" = "Bearer $($login.token)" }
        $me = Invoke-RestMethod -Uri "http://localhost:${API_PORT}/api/auth/me" -Headers $headers
        if ($me.user_id) { Log-Pass "GET /api/auth/me OK" }

        # Register second user with learner role
        $reg2 = Invoke-RestMethod -Method Post -Uri "http://localhost:${API_PORT}/api/auth/register" -ContentType "application/json" -Body '{"username":"testlearner","email":"tl@test.local","password":"Learner@2024!","display_name":"Test Learner","role":"learner"}'
        if ($reg2.message -match "learner") { Log-Pass "Second user registered with learner role" }

        # RBAC test
        $ll = Invoke-RestMethod -Method Post -Uri "http://localhost:${API_PORT}/api/auth/login" -ContentType "application/json" -Body '{"username":"testlearner","password":"Learner@2024!"}'
        $lh = @{ "Authorization" = "Bearer $($ll.token)" }
        try { Invoke-RestMethod -Uri "http://localhost:${API_PORT}/api/procurement/settlements" -Headers $lh; Log-Fail "Learner accessed settlements" }
        catch { if ($_.Exception.Response.StatusCode.value__ -eq 404) { Log-Pass "RBAC: Learner gets 404 (feature invisible)" } else { Log-Fail "RBAC: Got $($_.Exception.Response.StatusCode.value__)" } }
    }
} catch { Log-Fail "Login/API test failed: $_" }

Log-Info "Phase 4: Integration tests..."
$net = (docker network ls --format '{{.Name}}' | Select-String 'repo' | Select-Object -First 1).ToString().Trim()
docker run --rm --network=$net -e "TEST_DATABASE_URL=postgres://wlpr:wlpr_secret@db:5432/wlpr_portal?sslmode=disable" -v "${scriptDir}/backend:/src:ro" -v "${scriptDir}/config/config.js:/config/config.js:ro" golang:1.22-alpine sh -c "cp -r /src /app && cd /app && go mod tidy && go test -v -tags=integration -count=1 -timeout=120s ./internal/repository/ 2>&1"
if ($LASTEXITCODE -eq 0) { Log-Pass "Integration tests passed" } else { Log-Fail "Integration tests failed" }

Log-Info "Phase 5: Unit tests..."
docker run --rm -v "${scriptDir}/backend:/src:ro" golang:1.22-alpine sh -c "cp -r /src /app && cd /app && go mod tidy && go test -v -count=1 -timeout=120s ./pkg/... ./internal/services/ ./internal/handlers/ ./internal/middleware/ 2>&1"
if ($LASTEXITCODE -eq 0) { Log-Pass "Unit tests passed" } else { Log-Fail "Unit tests failed" }

Log-Info "Phase 6: Frontend tests..."
docker run --rm -v "${scriptDir}/frontend:/src:ro" node:20-alpine sh -c "cp -r /src /app && cd /app && npm install --legacy-peer-deps 2>&1 | tail -1 && npx vitest run 2>&1"
if ($LASTEXITCODE -eq 0) { Log-Pass "Frontend tests passed" } else { Log-Fail "Frontend tests failed" }

Log-Info "Phase 7: Cleanup..."
docker compose down -v --remove-orphans

if ($failed -eq 0) { Write-Host "`n  ALL TESTS PASSED" -ForegroundColor Green; exit 0 }
else { Write-Host "`n  SOME TESTS FAILED" -ForegroundColor Red; exit 1 }
