# Builds sing-box from source into build\bin\core\sing-box.exe, next to the
# NextBox.exe produced by `wails build` / `wails dev`.
# Usage:  powershell -ExecutionPolicy Bypass -File scripts\build-core.ps1 [-Source ..\sing-box-stable]

param(
    [string]$Source = (Join-Path $PSScriptRoot '..\..\sing-box-stable')
)

$ErrorActionPreference = 'Stop'
$Source = (Resolve-Path $Source).Path
$Out = Join-Path $PSScriptRoot '..\build\bin\core'
New-Item -ItemType Directory -Force $Out | Out-Null

# Same tags as the official Windows release, minus the naive outbound (needs an extra cronet DLL).
$tags = (Get-Content (Join-Path $Source 'release\DEFAULT_BUILD_TAGS_WINDOWS') -Raw).Trim() -split ',' |
    Where-Object { $_ -notin @('with_naive_outbound', 'with_purego') }
$tags = $tags -join ','

# Version from the first changelog heading, e.g. "#### 1.14.4"
$version = (Select-String -Path (Join-Path $Source 'docs\changelog.md') -Pattern '^#### (.+)$' | Select-Object -First 1).Matches[0].Groups[1].Value
$sharedLd = (Get-Content (Join-Path $Source 'release\LDFLAGS') -Raw).Trim()
$ldflags = "-X 'github.com/sagernet/sing-box/constant.Version=$version' $sharedLd -s -w -buildid="

Write-Host "Building sing-box $version"
Write-Host "Tags: $tags"

Push-Location $Source
try {
    $env:CGO_ENABLED = '0'
    go build -v -trimpath -tags $tags -ldflags $ldflags -o (Join-Path $Out 'sing-box.exe') ./cmd/sing-box
    if ($LASTEXITCODE -ne 0) { throw "go build failed ($LASTEXITCODE)" }
}
finally {
    Pop-Location
}
Write-Host "Done: $(Resolve-Path (Join-Path $Out 'sing-box.exe'))"
