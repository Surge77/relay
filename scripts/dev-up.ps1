# Starts the local infrastructure Relay depends on: PostgreSQL and a
# Redis-compatible server (Memurai). Run from an ELEVATED PowerShell — starting
# Windows services requires administrator rights.
#
#   Right-click PowerShell -> Run as administrator, then:
#   ./scripts/dev-up.ps1

$ErrorActionPreference = "Stop"

function Start-IfPresent($pattern, $label) {
    $svc = Get-Service -ErrorAction SilentlyContinue | Where-Object { $_.Name -match $pattern } | Select-Object -First 1
    if (-not $svc) {
        Write-Warning "$label service not found. Install it first (see README)."
        return
    }
    if ($svc.Status -ne 'Running') {
        Start-Service $svc.Name
        Write-Host "$label started ($($svc.Name))."
    } else {
        Write-Host "$label already running ($($svc.Name))."
    }
}

Start-IfPresent 'postgresql' 'PostgreSQL'
Start-IfPresent 'memurai'    'Memurai (Redis)'

Write-Host ""
Write-Host "Infra up. Next:"
Write-Host "  migrate -path migrations -database `$env:POSTGRES_URL up"
Write-Host "  cd gateway; go run ./cmd/gateway"
