# Copies the Go code NextBox for Windows shares with NextBox for Android into
# ..\NextBoxAndroid\core\nextbox\internal, so both apps build configs, read
# subscriptions and handle backups the same way.
#
#   powershell -ExecutionPolicy Bypass -File scripts\sync-android.ps1          # copy
#   powershell -ExecutionPolicy Bypass -File scripts\sync-android.ps1 -Check   # only report differences (exit 1)
param([switch]$Check)
$ErrorActionPreference = 'Stop'
$Root = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
$Android = Join-Path $Root '..\NextBoxAndroid\core\nextbox\internal'
if (-not (Test-Path $Android)) { throw "NextBox for Android not found at $Android" }
$Android = (Resolve-Path $Android).Path

# whole packages, and single files of packages that also have app-specific files
$dirs = @('clash', 'config', 'gost', 'link', 'ruleset', 'store', 'sub', 'vault')
$files = @(
    'geo\trace.go', 'geo\trace_test.go', 'geo\asn.go', 'geo\asn_test.go', 'geo\probe.go',
    'i18n\i18n.go'
)

$list = @()
foreach ($d in $dirs) {
    Get-ChildItem -Recurse -File (Join-Path $Root "internal\$d") | ForEach-Object {
        $list += $_.FullName.Substring((Join-Path $Root 'internal').Length + 1)
    }
}
$list += $files

$different = @()
foreach ($rel in $list) {
    $src = Join-Path $Root "internal\$rel"
    $dst = Join-Path $Android $rel
    $same = (Test-Path $dst) -and ((Get-FileHash $src).Hash -eq (Get-FileHash $dst).Hash)
    if ($same) { continue }
    $different += $rel
    if (-not $Check) {
        New-Item -ItemType Directory -Force (Split-Path $dst) | Out-Null
        Copy-Item $src $dst -Force
    }
}
# files the Android copy has that Windows no longer has, in the whole packages
foreach ($d in $dirs) {
    $dir = Join-Path $Android $d
    if (-not (Test-Path $dir)) { continue }
    Get-ChildItem -Recurse -File $dir | ForEach-Object {
        $rel = $_.FullName.Substring($Android.Length + 1)
        if (-not (Test-Path (Join-Path $Root "internal\$rel"))) {
            $different += "$rel (only on Android)"
        }
    }
}

if ($different.Count -eq 0) { Write-Host 'Shared code is the same in both apps.'; exit 0 }
if ($Check) {
    Write-Host "Shared code differs:"; $different | ForEach-Object { Write-Host "  $_" }
    exit 1
}
Write-Host "Copied to NextBox for Android:"; $different | ForEach-Object { Write-Host "  $_" }
