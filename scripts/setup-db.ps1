# Creates the relay database and role in the local Postgres instance, then runs
# migrations. Requires psql on PATH (C:\Program Files\PostgreSQL\17\bin) and the
# postgres superuser password.
#
#   ./scripts/setup-db.ps1 -SuperPassword <postgres-password>

param(
    [Parameter(Mandatory = $true)][string]$SuperPassword,
    [string]$RelayPassword = "relay"
)

$ErrorActionPreference = "Stop"
$env:PGPASSWORD = $SuperPassword

$sql = @"
DO `$`$ BEGIN
  IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'relay') THEN
    CREATE ROLE relay LOGIN PASSWORD '$RelayPassword';
  END IF;
END `$`$;
"@

psql -U postgres -h localhost -c $sql
psql -U postgres -h localhost -tc "SELECT 1 FROM pg_database WHERE datname='relay'" | Select-String 1 -Quiet | Out-Null
psql -U postgres -h localhost -c "CREATE DATABASE relay OWNER relay;" 2>$null

Write-Host "Database ready. Now run:"
Write-Host "  migrate -path migrations -database `"postgres://relay:$RelayPassword@localhost:5432/relay?sslmode=disable`" up"
