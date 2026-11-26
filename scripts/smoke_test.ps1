<#
.SYNOPSIS
    End-to-end API smoke test against running native QUANTA services.
.DESCRIPTION
    Verifies live HTTP responses across AI Engine (8000), Optimization Engine (8001),
    Go Backend Gateway (8080), JWT Auth (register/login), Optimization Comparison,
    and Adversarial + Scenario Stress Testing.
#>
[CmdletBinding()]
param(
    [string]$AiUrl = 'http://localhost:8000',
    [string]$OptUrl = 'http://localhost:8001',
    [string]$BackendUrl = 'http://localhost:8080',
    [string]$AdminEmail = 'smoke_admin@quanta.local',
    [string]$AdminPassword = 'QuantaSmokePass!2026',
    [string]$AnalystEmail = 'smoke_analyst@quanta.local',
    [string]$AnalystPassword = 'QuantaSmokePass!2026'
)

$ErrorActionPreference = 'Stop'

function Pass-Step([string]$Name, [string]$Detail) {
    Write-Host "[PASS] $Name - $Detail" -ForegroundColor Green
}

function Fail-Step([string]$Name, [string]$Detail) {
    Write-Host "[FAIL] $Name - $Detail" -ForegroundColor Red
    throw "$Name failed: $Detail"
}

function Invoke-Api {
    param(
        [string]$Method,
        [string]$Uri,
        [object]$Body = $null,
        [hashtable]$Headers = @{},
        [int[]]$AllowedStatusCodes = @(200, 201)
    )
    $params = @{
        Method      = $Method
        Uri         = $Uri
        Headers     = $Headers
        ContentType = 'application/json'
        TimeoutSec  = 30
    }
    if ($null -ne $Body) {
        $params['Body'] = ($Body | ConvertTo-Json -Depth 10 -Compress)
    }

    try {
        $resp = Invoke-RestMethod @params
        return @{ Status = 200; Data = $resp }
    } catch {
        $webResp = $_.Exception.Response
        if ($webResp) {
            $code = [int]$webResp.StatusCode
            $rawBody = ''
            try {
                $stream = $webResp.GetResponseStream()
                if ($stream) {
                    $reader = New-Object System.IO.StreamReader($stream)
                    $rawBody = $reader.ReadToEnd()
                    $reader.Dispose()
                }
            } catch {}

            if ($AllowedStatusCodes -contains $code) {
                $parsed = $null
                if ($rawBody) { try { $parsed = $rawBody | ConvertFrom-Json } catch { $parsed = $rawBody } }
                return @{ Status = $code; Data = $parsed }
            }
            throw "HTTP $code from $Method $Uri : $rawBody"
        }
        throw
    }
}

Write-Host "=== QUANTA End-to-End Smoke Test ===" -ForegroundColor Cyan

# 1. Health Checks
$aiHealth = (Invoke-Api -Method 'GET' -Uri "$AiUrl/health").Data
if ($aiHealth.status -ne 'healthy') { Fail-Step 'AI Engine Health' "Unexpected status: $($aiHealth | ConvertTo-Json -Compress)" }
Pass-Step 'AI Engine Health' "$AiUrl/health -> status=$($aiHealth.status)"

$optHealth = (Invoke-Api -Method 'GET' -Uri "$OptUrl/health").Data
if ($optHealth.status -ne 'healthy') { Fail-Step 'Optimization Engine Health' "Unexpected status: $($optHealth | ConvertTo-Json -Compress)" }
Pass-Step 'Optimization Engine Health' "$OptUrl/health -> status=$($optHealth.status)"

$gwHealth = (Invoke-Api -Method 'GET' -Uri "$BackendUrl/api/v1/health").Data
if ($gwHealth.status -ne 'ok') { Fail-Step 'Go Gateway Health' "Unexpected status: $($gwHealth | ConvertTo-Json -Compress)" }
Pass-Step 'Go Gateway Health' "$BackendUrl/api/v1/health -> db=$($gwHealth.database), ai=$($gwHealth.dependencies.ai_engine), opt=$($gwHealth.dependencies.optimization_engine)"

# 2. Auth Register & Login (bootstrap admin / analyst role)
$regAdmin = Invoke-Api -Method 'POST' -Uri "$BackendUrl/api/v1/auth/register" -Body @{
    email     = $AdminEmail
    password  = $AdminPassword
    full_name = 'Smoke Test Admin'
} -AllowedStatusCodes @(200, 201, 409)

$adminLogin = (Invoke-Api -Method 'POST' -Uri "$BackendUrl/api/v1/auth/login" -Body @{
    email    = $AdminEmail
    password = $AdminPassword
}).Data

if (-not $adminLogin.token) { Fail-Step 'Admin Login' 'Missing JWT token in login response' }
Pass-Step 'Auth Bootstrap Login' "Logged in as $($adminLogin.user.email) (role=$($adminLogin.user.role))"

$regAnalyst = Invoke-Api -Method 'POST' -Uri "$BackendUrl/api/v1/auth/register" -Body @{
    email     = $AnalystEmail
    password  = $AnalystPassword
    full_name = 'Smoke Test Analyst'
} -AllowedStatusCodes @(200, 201, 409)

$analystLogin = (Invoke-Api -Method 'POST' -Uri "$BackendUrl/api/v1/auth/login" -Body @{
    email    = $AnalystEmail
    password = $AnalystPassword
}).Data

$token = $analystLogin.token
$activeRole = $analystLogin.user.role
if ($activeRole -eq 'viewer' -and $adminLogin.user.role -eq 'admin') {
    $promoted = (Invoke-Api -Method 'PATCH' -Uri "$BackendUrl/api/v1/users/$($analystLogin.user.id)/role" `
        -Headers @{ Authorization = "Bearer $($adminLogin.token)" } `
        -Body @{ role = 'analyst' }).Data
    $analystLogin = (Invoke-Api -Method 'POST' -Uri "$BackendUrl/api/v1/auth/login" -Body @{
        email    = $AnalystEmail
        password = $AnalystPassword
    }).Data
    $token = $analystLogin.token
    $activeRole = $analystLogin.user.role
}
if ($activeRole -notin @('analyst', 'admin')) {
    Write-Host "[WARN] Analyst account has role '$activeRole'; falling back to primary login token ($($adminLogin.user.role))." -ForegroundColor Yellow
    $token = $adminLogin.token
}
Pass-Step 'Analyst JWT Auth' "Active token role='$activeRole'"

$authHeaders = @{ Authorization = "Bearer $token" }

# 3. Classical vs. Quantum Optimization Compare via Go Gateway
$optProblem = @{
    problem = @{
        capital           = 100.0
        step_size         = 20.0
        risk_tolerance    = 0.5
        minimum_liquidity = 0.25
        objective         = 'maximize_risk_adjusted_return'
        market_state      = 'normal'
        assets            = @(
            @{ id = 'gov_bond';   name = 'Sovereign G-Sec';  asset_type = 'gov_bond';  cost = 20.0; expected_return = 0.072; risk = 0.04; liquidity = 0.94 }
            @{ id = 'corp_bond';  name = 'AAA Corp Bond';    asset_type = 'corp_bond'; cost = 20.0; expected_return = 0.092; risk = 0.11; liquidity = 0.78 }
            @{ id = 'equity';     name = 'Bluechip Equity';  asset_type = 'equity';    cost = 25.0; expected_return = 0.148; risk = 0.34; liquidity = 0.82 }
            @{ id = 'tbill';      name = '91-Day T-Bills';   asset_type = 'cash';      cost = 15.0; expected_return = 0.038; risk = 0.01; liquidity = 0.99 }
        )
        liabilities       = @(
            @{ id = 'loan_1'; name = 'Senior Facility'; principal = 15.0; amount = 15.0; interest_rate = 0.08; maturity = 12; priority = 1; is_mandatory_reserve = $true }
        )
        constraints       = @(
            @{ id = 'c_liq'; constraint_type = 'minimum_liquidity'; parameters = @{ threshold = 0.25 } }
        )
    }
    options = @{
        p     = 2
        shots = 1024
        seed  = 42
    }
}

$compareResp = (Invoke-Api -Method 'POST' -Uri "$BackendUrl/api/v1/optimization/compare" -Headers $authHeaders -Body $optProblem).Data
if (-not $compareResp.best_classical_method -or -not $compareResp.results.qaoa) {
    Fail-Step 'Optimization Compare' 'Missing best_classical_method or qaoa result'
}
Pass-Step 'Optimization Compare (Go -> Port 8001)' "best_classical=$($compareResp.best_classical_method), qaoa_ratio=$($compareResp.qaoa_approximation_ratio), qaoa_ms=$([math]::Round([double]$compareResp.results.qaoa.runtime_ms, 2))"

# 4. Stress-Test End-to-End (AI Engine Direct Alias + Go Gateway Treasury Stress Test)
$aiStressProblem = @{
    capital           = 100.0
    step_size         = 25.0
    risk_tolerance    = 0.5
    minimum_liquidity = 0.20
    assets            = $optProblem.problem.assets
    liabilities       = @(
        @{ id = 'loan_1'; name = 'Senior Facility'; principal = 15.0; interest_rate = 0.08; maturity_months = 12; priority = 1; is_mandatory_reserve = $true }
    )
    constraints       = $optProblem.problem.constraints
}

$aiStressResp = (Invoke-Api -Method 'POST' -Uri "$AiUrl/api/v1/stress-test" -Body $aiStressProblem).Data
if ($null -eq $aiStressResp.worst_case_payoff -or $aiStressResp.nodes_evaluated -le 0) {
    Fail-Step 'AI Stress-Test Alias' 'Invalid /api/v1/stress-test response'
}
Pass-Step 'AI Stress-Test Alias (Port 8000)' "shock='$($aiStressResp.worst_market_shock)', payoff=$([math]::Round([double]$aiStressResp.worst_case_payoff, 4)), nodes=$($aiStressResp.nodes_evaluated)"

$createdTreasury = (Invoke-Api -Method 'POST' -Uri "$BackendUrl/api/v1/treasuries" -Headers $authHeaders -Body @{
    name                = 'Smoke Test Treasury'
    base_currency       = 'INR'
    total_capital       = 100.0
    risk_tolerance      = 0.5
    min_liquidity_ratio = 0.2
}).Data

$gwStressResp = (Invoke-Api -Method 'POST' -Uri "$BackendUrl/api/v1/treasuries/$($createdTreasury.id)/stress-test" -Headers $authHeaders -Body @{
    assets      = $optProblem.problem.assets
    liabilities = $aiStressProblem.liabilities
    constraints = $optProblem.problem.constraints
    step_size   = 25.0
}).Data

if (-not $gwStressResp.adversarial_result -or -not $gwStressResp.scenarios) {
    Fail-Step 'Go Gateway Stress-Test' 'Missing adversarial_result or scenarios'
}
Pass-Step 'Go Gateway Stress-Test (Go -> Port 8000)' "treasury=$($createdTreasury.id), scenarios=$($gwStressResp.scenarios.Count), worst_shock='$($gwStressResp.adversarial_result.worst_market_shock)'"

Write-Host ""
Write-Host "ALL SMOKE TESTS PASSED SUCCESSFULLY." -ForegroundColor Green
