# Builds the NextBox service (cmd/nextbox-service) into
# build\bin\core\NextBoxService.exe, next to the core it installs together
# with. Its version must be NextBox's (main.go), which NextBox compares with
# the installed service to offer "更新服务".
# Usage:  powershell -ExecutionPolicy Bypass -File scripts\build-service.ps1

$ErrorActionPreference = 'Stop'
$Root = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
$Out = Join-Path $Root 'build\bin\core'
New-Item -ItemType Directory -Force $Out | Out-Null

$version = (Select-String -Path (Join-Path $Root 'main.go') -Pattern '^const Version = "(.+)"$').Matches[0].Groups[1].Value
Write-Host "Building NextBoxService $version"

Push-Location $Root
try {
    $env:CGO_ENABLED = '0'
    $env:GOOS = 'windows'
    $env:GOARCH = 'amd64'
    go build -trimpath -ldflags "-s -w -buildid= -H windowsgui -X main.Version=$version" -o (Join-Path $Out 'NextBoxService.exe') ./cmd/nextbox-service
    if ($LASTEXITCODE -ne 0) { throw "go build failed ($LASTEXITCODE)" }
}
finally {
    Pop-Location
}
Write-Host "Done: $(Join-Path $Out 'NextBoxService.exe')"
