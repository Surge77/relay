# Launches the full multi-node Relay stack locally with ZERO admin and no Docker:
#   - a user-owned Postgres cluster (no Windows service, no elevation)
#   - devredis (bundled miniredis) as a shared Redis broker on :6379
#   - two gateway nodes (:8080, :8081) sharing that Postgres + Redis
#   - the Next.js web client (:3000)
#
# Open two browser tabs to demonstrate cross-node fan-out:
#   alice -> http://localhost:3000/?gw=ws://localhost:8080/ws   (node A)
#   bob   -> http://localhost:3000/?gw=ws://localhost:8081/ws   (node B, pick "bob")
#
# NOT for production. The JWT secret here is a throwaway dev value.

$ErrorActionPreference = "Stop"
$root   = Split-Path $PSScriptRoot -Parent
$pgbin  = "C:\Program Files\PostgreSQL\17\bin"
$datadir = "$env:USERPROFILE\relay-pgdata"
$secret = "dev-relay-manual-test-secret"
$pgurl  = "postgres://postgres@localhost:5432/relay?sslmode=disable"

function Test-Port($port) { [bool](Get-NetTCPConnection -State Listen -LocalPort $port -ErrorAction SilentlyContinue) }

# 1. Postgres (user-owned cluster).
if (-not (Test-Path "$datadir\PG_VERSION")) {
    & "$pgbin\initdb.exe" -D $datadir -U postgres -A trust -E UTF8 | Out-Null
}
if (-not (Test-Port 5432)) {
    & "$pgbin\pg_ctl.exe" -D $datadir -l "$datadir\server.log" -o "-p 5432" start | Out-Null
    Start-Sleep -Seconds 2
}
if ((& "$pgbin\psql.exe" -U postgres -h localhost -p 5432 -d postgres -tAc "SELECT 1 FROM pg_database WHERE datname='relay'") -ne "1") {
    & "$pgbin\createdb.exe" -U postgres -h localhost -p 5432 relay
}
& migrate -path "$root\migrations" -database $pgurl up

# 2. devredis broker.
if (-not (Test-Port 6379)) {
    Start-Process pwsh -ArgumentList "-NoExit","-Command","Set-Location '$root\gateway'; go run ./cmd/devredis -addr 127.0.0.1:6379"
    while (-not (Test-Port 6379)) { Start-Sleep -Milliseconds 500 }
}

# 3. Two gateway nodes sharing the same Postgres + Redis.
$env:JWT_SECRET = $secret
$env:POSTGRES_URL = $pgurl
$env:REDIS_URL = "redis://localhost:6379"
$env:ALLOWED_ORIGINS = "http://localhost:3000"
foreach ($n in @(@{id="gwA";port=8080}, @{id="gwB";port=8081})) {
    Start-Process pwsh -ArgumentList "-NoExit","-Command",
        "`$env:JWT_SECRET='$secret'; `$env:POSTGRES_URL='$pgurl'; `$env:REDIS_URL='redis://localhost:6379'; `$env:ALLOWED_ORIGINS='http://localhost:3000'; `$env:NODE_ID='$($n.id)'; `$env:GATEWAY_PORT='$($n.port)'; Set-Location '$root\gateway'; go run ./cmd/gateway"
}

# 4. Web client.
Start-Process pwsh -ArgumentList "-NoExit","-Command",
    "`$env:JWT_SECRET='$secret'; Set-Location '$root\web'; npm run dev"

Write-Host ""
Write-Host "Relay multi-node stack starting. Once all windows are ready:"
Write-Host "  alice (node A): http://localhost:3000/?gw=ws://localhost:8080/ws"
Write-Host "  bob   (node B): http://localhost:3000/?gw=ws://localhost:8081/ws  (select 'bob')"
