<#
.SYNOPSIS
    Cleanly stops all native QUANTA processes listening on ports 8000, 8001, 8080, and 3000.
#>
[CmdletBinding()]
param()

$ErrorActionPreference = 'SilentlyContinue'

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$RepoRoot = (Resolve-Path (Join-Path $ScriptDir '..')).Path
$LogDir = Join-Path $RepoRoot '.logs'

$targetPorts = @(8000, 8001, 8080, 3000)
$stoppedPids = New-Object 'System.Collections.Generic.HashSet[int]'

Write-Host "[QUANTA] Stopping native local processes on ports $($targetPorts -join ', ')..." -ForegroundColor Cyan

foreach ($port in $targetPorts) {
    $connections = Get-NetTCPConnection -LocalPort $port -State Listen -ErrorAction SilentlyContinue
    if ($connections) {
        $procIds = $connections | Select-Object -ExpandProperty OwningProcess -Unique
        foreach ($procId in $procIds) {
            if ($procId -and $procId -gt 4 -and -not $stoppedPids.Contains($procId)) {
                $proc = Get-Process -Id $procId -ErrorAction SilentlyContinue
                $procName = if ($proc) { $proc.ProcessName } else { 'unknown' }
                try {
                    Stop-Process -Id $procId -Force -ErrorAction Stop
                    $null = $stoppedPids.Add($procId)
                    Write-Host "[OK]     Stopped $procName (PID $procId) listening on port $port" -ForegroundColor Green
                } catch {
                    Write-Host "[WARN]   Failed to stop PID $procId on port ${port}: $($_.Exception.Message)" -ForegroundColor Yellow
                }
            }
        }
    }
}

# Also clean up launcher shell processes recorded in .logs/*.pid
if (Test-Path $LogDir) {
    $pidFiles = Get-ChildItem -Path $LogDir -Filter '*.pid' -ErrorAction SilentlyContinue
    foreach ($pf in $pidFiles) {
        $rawPid = (Get-Content -Path $pf.FullName -ErrorAction SilentlyContinue | Select-Object -First 1)
        if ($rawPid -match '^\d+$') {
            $launcherPid = [int]$rawPid
            if ($launcherPid -gt 4 -and -not $stoppedPids.Contains($launcherPid)) {
                $proc = Get-Process -Id $launcherPid -ErrorAction SilentlyContinue
                if ($proc) {
                    Stop-Process -Id $launcherPid -Force -ErrorAction SilentlyContinue
                    $null = $stoppedPids.Add($launcherPid)
                    Write-Host "[OK]     Stopped launcher process $($proc.ProcessName) (PID $launcherPid) from $($pf.Name)" -ForegroundColor Green
                }
            }
        }
        Remove-Item -Path $pf.FullName -Force -ErrorAction SilentlyContinue
    }
}

if ($stoppedPids.Count -eq 0) {
    Write-Host "[QUANTA] No active processes found listening on QUANTA ports." -ForegroundColor Gray
} else {
    Write-Host "[QUANTA] Shutdown complete ($($stoppedPids.Count) process(es) stopped)." -ForegroundColor Cyan
}
