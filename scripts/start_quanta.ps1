<#
.SYNOPSIS
    Starts the complete QUANTA platform natively on Windows (no Docker required).
.DESCRIPTION
    1. Loads environment variables from root .env into the process environment.
    2. Verifies PostgreSQL is listening on localhost:5432 (attempting to start a stopped Windows PostgreSQL service)
       and runs backend SQL migrations via psql if psql is available.
    3. Launches AI Engine (8000), Optimization Engine (8001), Go Backend (8080), and SvelteKit Frontend (3000)
       in labeled PowerShell windows (or background processes with -Background).
    4. Polls /health endpoints until services respond healthy and prints local URLs.
#>
[CmdletBinding()]
param(
    [switch]$Background,
    [int]$HealthTimeoutSeconds = 90
)

$ErrorActionPreference = 'Stop'

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$RepoRoot = (Resolve-Path (Join-Path $ScriptDir '..')).Path
$LogDir = Join-Path $RepoRoot '.logs'

if (-not (Test-Path $LogDir)) {
    New-Item -ItemType Directory -Path $LogDir -Force | Out-Null
}

function Write-Info([string]$Msg)  { Write-Host "[QUANTA] $Msg" -ForegroundColor Cyan }
function Write-Ok([string]$Msg)    { Write-Host "[OK]     $Msg" -ForegroundColor Green }
function Write-Warn([string]$Msg)  { Write-Host "[WARN]   $Msg" -ForegroundColor Yellow }
function Write-Fail([string]$Msg)  { Write-Host "[ERROR]  $Msg" -ForegroundColor Red }

# 1. Load .env variables into process environment
$EnvFile = Join-Path $RepoRoot '.env'
if (Test-Path $EnvFile) {
    Write-Info "Loading environment from $EnvFile"
    foreach ($rawLine in Get-Content -Path $EnvFile) {
        $line = $rawLine.Trim()
        if ([string]::IsNullOrWhiteSpace($line) -or $line.StartsWith('#')) {
            continue
        }
        $eqIdx = $line.IndexOf('=')
        if ($eqIdx -lt 1) {
            continue
        }
        $key = $line.Substring(0, $eqIdx).Trim()
        $val = $line.Substring($eqIdx + 1).Trim()
        if (($val.StartsWith('"') -and $val.EndsWith('"')) -or ($val.StartsWith("'") -and $val.EndsWith("'"))) {
            if ($val.Length -ge 2) {
                $val = $val.Substring(1, $val.Length - 2)
            }
        }
        [Environment]::SetEnvironmentVariable($key, $val, 'Process')
    }
} else {
    Write-Warn "Root .env file not found at $EnvFile; using existing process defaults."
}

function Test-TcpPort([string]$TargetHost, [int]$Port, [int]$TimeoutMs = 1500) {
    $client = New-Object System.Net.Sockets.TcpClient
    try {
        $iar = $client.BeginConnect($TargetHost, $Port, $null, $null)
        if (-not $iar.AsyncWaitHandle.WaitOne($TimeoutMs, $false)) {
            return $false
        }
        $client.EndConnect($iar)
        return $true
    } catch {
        return $false
    } finally {
        $client.Dispose()
    }
}

# 2. Verify PostgreSQL on localhost:5432
$pgHost = if ($env:POSTGRES_HOST) { $env:POSTGRES_HOST } else { 'localhost' }
$pgPort = if ($env:POSTGRES_PORT) { [int]$env:POSTGRES_PORT } else { 5432 }

Write-Info "Checking PostgreSQL reachability on ${pgHost}:${pgPort}..."
if (-not (Test-TcpPort -TargetHost $pgHost -Port $pgPort)) {
    Write-Warn "PostgreSQL is not reachable on ${pgHost}:${pgPort}. Checking Windows services..."
    try {
        $pgServices = Get-Service -Name '*postgres*' -ErrorAction SilentlyContinue
        if ($pgServices) {
            foreach ($svc in $pgServices) {
                if ($svc.Status -ne 'Running') {
                    Write-Info "Attempting to start Windows service '$($svc.Name)'..."
                    Start-Service -Name $svc.Name -ErrorAction Stop
                }
            }
            Start-Sleep -Seconds 2
        }
    } catch {
        Write-Warn "Could not auto-start Windows PostgreSQL service (may require Administrator rights): $($_.Exception.Message)"
    }
}

if (-not (Test-TcpPort -TargetHost $pgHost -Port $pgPort)) {
    Write-Fail "PostgreSQL is NOT reachable on ${pgHost}:${pgPort}."
    Write-Host "  Next steps:" -ForegroundColor Yellow
    Write-Host "  1. Start your local PostgreSQL service (e.g., services.msc -> postgresql-x64-*)."
    Write-Host "  2. Initialize role/database with: psql -U postgres -f scripts\init_db.sql"
    exit 1
}
Write-Ok "PostgreSQL TCP socket is reachable on ${pgHost}:${pgPort}."

# Locate psql if installed (PATH or standard PostgreSQL Windows folder)
function Find-Psql {
    $cmd = Get-Command psql -ErrorAction SilentlyContinue
    if ($cmd) { return $cmd.Source }
    $candidates = Get-ChildItem -Path 'C:\Program Files\PostgreSQL\*\bin\psql.exe' -ErrorAction SilentlyContinue | Sort-Object FullName -Descending
    if ($candidates) { return $candidates[0].FullName }
    return $null
}

$psqlExe = Find-Psql
$dbUrl = $env:DATABASE_URL
if ($psqlExe -and $dbUrl) {
    Write-Info "Applying SQL migrations via psql ($psqlExe)..."
    $migrationFiles = @(
        'apps\backend\migrations\000_bootstrap.sql',
        'apps\backend\migrations\001_step5_tables.sql',
        'apps\backend\migrations\002_users.sql',
        'apps\backend\migrations\003_treasury_owner.sql',
        'apps\backend\migrations\004_treasury_assets_liabilities.sql',
        'apps\backend\migrations\005_audit_logs_entity_id_text.sql'
    )
    $migrationOk = $true
    foreach ($rel in $migrationFiles) {
        $fullSql = Join-Path $RepoRoot $rel
        if (Test-Path $fullSql) {
            $out = & $psqlExe $dbUrl -v ON_ERROR_STOP=1 -f $fullSql 2>&1
            if ($LASTEXITCODE -ne 0) {
                $migrationOk = $false
                Write-Warn "Migration $rel returned non-zero status (Go backend will also attempt migrations on connect): $out"
                break
            }
        }
    }
    if ($migrationOk) {
        Write-Ok "PostgreSQL migrations 000-005 verified/applied."
    }
} elseif (-not $psqlExe) {
    Write-Warn "psql.exe not found on PATH; skipping manual SQL step (Go backend applies migrations automatically on connect)."
}

# 3. Resolve Python, Go, npm executables
$pythonCmd = Get-Command python -ErrorAction SilentlyContinue
$pythonExe = if ($pythonCmd) { $pythonCmd.Source } elseif (Test-Path 'C:\Python315\python.exe') { 'C:\Python315\python.exe' } else { $null }
if (-not $pythonExe) {
    Write-Fail "Python executable not found on PATH."
    exit 1
}

$goCmd = Get-Command go -ErrorAction SilentlyContinue
if (-not $goCmd) {
    Write-Fail "Go toolchain ('go') not found on PATH."
    exit 1
}

$npmCmd = Get-Command npm.cmd -ErrorAction SilentlyContinue
if (-not $npmCmd) { $npmCmd = Get-Command npm -ErrorAction SilentlyContinue }
if (-not $npmCmd) {
    Write-Fail "Node/npm ('npm') not found on PATH."
    exit 1
}

$frontendDir = Join-Path $RepoRoot 'apps\frontend'
if (-not (Test-Path (Join-Path $frontendDir 'node_modules'))) {
    Write-Info "Installing frontend npm dependencies (node_modules missing)..."
    Push-Location $frontendDir
    try {
        & $npmCmd.Source ci
        if ($LASTEXITCODE -ne 0) {
            Write-Fail "Frontend 'npm ci' failed."
            exit 1
        }
    } finally {
        Pop-Location
    }
}

# Helper to launch a service cleanly
$services = @(
    @{
        Key     = 'ai-engine'
        Title   = 'QUANTA - AI Decision Engine (Port 8000)'
        WorkDir = (Join-Path $RepoRoot 'services\ai-engine')
        Command = "& '$pythonExe' -m uvicorn app.main:app --host 0.0.0.0 --port 8000"
    },
    @{
        Key     = 'optimization-engine'
        Title   = 'QUANTA - Optimization & QAOA Engine (Port 8001)'
        WorkDir = (Join-Path $RepoRoot 'services\optimization-engine')
        Command = "& '$pythonExe' -m uvicorn app.main:app --host 0.0.0.0 --port 8001"
    },
    @{
        Key     = 'backend'
        Title   = 'QUANTA - Go Backend Gateway (Port 8080)'
        WorkDir = (Join-Path $RepoRoot 'apps\backend')
        Command = "& '$($goCmd.Source)' run ./cmd/server"
    },
    @{
        Key     = 'frontend'
        Title   = 'QUANTA - SvelteKit Dashboard (Port 3000)'
        WorkDir = $frontendDir
        Command = "& '$($npmCmd.Source)' run dev -- --port 3000"
    }
)

$psExe = if (Get-Command pwsh -ErrorAction SilentlyContinue) { (Get-Command pwsh).Source } else { 'powershell.exe' }

foreach ($svc in $services) {
    $pidFile = Join-Path $LogDir "$($svc.Key).pid"
    Write-Info "Starting $($svc.Title)..."
    if ($Background) {
        $outLog = Join-Path $LogDir "$($svc.Key).out.log"
        $errLog = Join-Path $LogDir "$($svc.Key).err.log"
        $proc = Start-Process -FilePath $psExe `
            -WorkingDirectory $svc.WorkDir `
            -ArgumentList @('-NoProfile', '-ExecutionPolicy', 'Bypass', '-Command', $svc.Command) `
            -RedirectStandardOutput $outLog `
            -RedirectStandardError $errLog `
            -WindowStyle Hidden `
            -PassThru
    } else {
        $windowScript = "`$Host.UI.RawUI.WindowTitle = '$($svc.Title)'; $($svc.Command)"
        $proc = Start-Process -FilePath $psExe `
            -WorkingDirectory $svc.WorkDir `
            -ArgumentList @('-NoExit', '-NoProfile', '-ExecutionPolicy', 'Bypass', '-Command', $windowScript) `
            -WindowStyle Normal `
            -PassThru
    }
    Set-Content -Path $pidFile -Value $proc.Id -Encoding Ascii
}

# 4. Poll /health endpoints until all backend services report healthy
$healthTargets = @(
    @{ Name = 'AI Engine';           Url = 'http://localhost:8000/health' },
    @{ Name = 'Optimization Engine'; Url = 'http://localhost:8001/health' },
    @{ Name = 'Go Backend Gateway';  Url = 'http://localhost:8080/api/v1/health' }
)

$deadline = (Get-Date).AddSeconds($HealthTimeoutSeconds)
$healthyMap = @{}

Write-Info "Waiting up to ${HealthTimeoutSeconds}s for service health checks..."
while ((Get-Date) -lt $deadline) {
    foreach ($target in $healthTargets) {
        if ($healthyMap[$target.Name]) { continue }
        try {
            $resp = Invoke-WebRequest -Uri $target.Url -UseBasicParsing -TimeoutSec 3 -ErrorAction Stop
            if ($resp.StatusCode -eq 200) {
                $healthyMap[$target.Name] = $true
                Write-Ok "$($target.Name) is healthy ($($target.Url))"
            }
        } catch {
            # Wait and retry
        }
    }

    if (-not $healthyMap['Frontend'] -and (Test-TcpPort -TargetHost 'localhost' -Port 3000 -TimeoutMs 1000)) {
        $healthyMap['Frontend'] = $true
        Write-Ok "SvelteKit Frontend dev server is listening (http://localhost:3000)"
    }

    if ($healthyMap.Count -ge 4) {
        break
    }
    Start-Sleep -Seconds 2
}

Write-Host ""
if ($healthyMap.Count -eq 4) {
    Write-Ok "All 4 QUANTA native services are up and running!"
} else {
    Write-Warn "Timed out waiting for all services. Healthy: $($healthyMap.Keys -join ', ')"
}

Write-Host ""
Write-Host "  QUANTA Local URLs (No Docker):" -ForegroundColor White
Write-Host "  - Frontend Dashboard : http://localhost:3000" -ForegroundColor Green
Write-Host "  - Go API Gateway     : http://localhost:8080/api/v1/health" -ForegroundColor Green
Write-Host "  - AI Decision Engine : http://localhost:8000/docs" -ForegroundColor Green
Write-Host "  - Optimization Lab   : http://localhost:8001/docs" -ForegroundColor Green
Write-Host ""
Write-Host "  Verify live APIs : powershell -ExecutionPolicy Bypass -File .\scripts\smoke_test.ps1" -ForegroundColor Gray
Write-Host "  Stop all servers : powershell -ExecutionPolicy Bypass -File .\scripts\stop_quanta.ps1" -ForegroundColor Gray
