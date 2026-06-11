[CmdletBinding()]
param(
    [switch]$Uninstall
)

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

$repoRoot = Resolve-Path (Join-Path $PSScriptRoot "..")
$gitDir = (& git -C $repoRoot rev-parse --git-dir).Trim()
if ($LASTEXITCODE -ne 0 -or [string]::IsNullOrWhiteSpace($gitDir)) {
    throw "Unable to locate .git directory"
}

if (-not [System.IO.Path]::IsPathRooted($gitDir)) {
    $gitDir = Join-Path $repoRoot $gitDir
}

$hooksDir = Join-Path $gitDir "hooks"
New-Item -ItemType Directory -Force -Path $hooksDir | Out-Null

$hookNames = @("post-commit", "post-merge")

if ($Uninstall) {
    foreach ($name in $hookNames) {
        $path = Join-Path $hooksDir $name
        if (Test-Path -LiteralPath $path) {
            Remove-Item -LiteralPath $path -Force
            Write-Host "Removed $path"
        }
    }
    return
}

$hook = @'
#!/bin/sh

branch="$(git rev-parse --abbrev-ref HEAD 2>/dev/null)"
if [ "$branch" != "main" ]; then
  exit 0
fi

case "$FUSION_GIT_DESK_SKIP_LOCAL_DEPLOY" in
  1|true|TRUE|yes|YES)
    echo "Skipping Fusion Git Desk local deploy because FUSION_GIT_DESK_SKIP_LOCAL_DEPLOY is set."
    exit 0
    ;;
esac

if command -v pwsh >/dev/null 2>&1; then
  pwsh -NoProfile -ExecutionPolicy Bypass -File "scripts/deploy-local.ps1"
elif command -v powershell.exe >/dev/null 2>&1; then
  powershell.exe -NoProfile -ExecutionPolicy Bypass -File "scripts/deploy-local.ps1"
else
  echo "PowerShell was not found; cannot run Fusion Git Desk local deploy."
  exit 1
fi
'@

foreach ($name in $hookNames) {
    $path = Join-Path $hooksDir $name
    Set-Content -LiteralPath $path -Value $hook -NoNewline -Encoding ASCII
    Write-Host "Installed $path"
}

Write-Host "Local auto deploy is enabled for main commits and merges."
Write-Host "Set FUSION_GIT_DESK_SKIP_LOCAL_DEPLOY=1 to skip a run."
