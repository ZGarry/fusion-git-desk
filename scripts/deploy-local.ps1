[CmdletBinding()]
param(
    [switch]$SkipTests,
    [switch]$SkipWailsBuild,
    [switch]$RegenerateBindings
)

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

$repoRoot = Resolve-Path (Join-Path $PSScriptRoot "..")

function Require-Command {
    param([Parameter(Mandatory = $true)][string]$Name)

    if (-not (Get-Command $Name -ErrorAction SilentlyContinue)) {
        throw "Required command not found: $Name"
    }
}

function Invoke-Step {
    param(
        [Parameter(Mandatory = $true)][string]$Name,
        [Parameter(Mandatory = $true)][scriptblock]$Script
    )

    Write-Host "==> $Name"
    & $Script
    if ($LASTEXITCODE -ne 0) {
        throw "$Name failed with exit code $LASTEXITCODE"
    }
}

Require-Command pnpm
Require-Command go
if (-not $SkipWailsBuild) {
    Require-Command wails
}

Push-Location $repoRoot
try {
    Invoke-Step "Build frontend" {
        Push-Location (Join-Path $repoRoot "frontend")
        try {
            pnpm build
        }
        finally {
            Pop-Location
        }
    }

    if (-not $SkipTests) {
        Invoke-Step "Run Go tests" {
            go test ./...
        }
    }

    if (-not $SkipWailsBuild) {
        Invoke-Step "Build Windows app" {
            if ($RegenerateBindings) {
                wails build
            }
            else {
                wails build -skipbindings
            }
        }
    }

    Write-Host "Local deployment complete."
    Write-Host ("Windows artifact: {0}" -f (Join-Path $repoRoot "build\bin\FusionGitDesk.exe"))
}
finally {
    Pop-Location
}
