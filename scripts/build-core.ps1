# Builds sing-box from source into build\bin\core\sing-box.exe, next to the
# NextBox.exe produced by `wails build` / `wails dev`. Only what NextBox uses
# is compiled in (see $tags below): the official Windows release also has
# Tailscale, cloudflared, OpenVPN / OpenConnect, USB/IP, ACME and the CCM /
# OCM services, which NextBox never configures and which make the core more
# than twice as large. The NaiveProxy outbound loads
# libcronet.dll at run time; it is copied next to sing-box.exe from the
# cronet-go module that the sing-box source pins (as the official release
# workflow does).
# Usage:  powershell -ExecutionPolicy Bypass -File scripts\build-core.ps1 [-Source ..\sing-box-stable]

param(
    [string]$Source = (Join-Path $PSScriptRoot '..\..\sing-box-stable')
)

$ErrorActionPreference = 'Stop'
$Source = (Resolve-Path $Source).Path
$Out = Join-Path $PSScriptRoot '..\build\bin\core'
New-Item -ItemType Directory -Force $Out | Out-Null

# gvisor: the gVisor / Mixed TUN stacks; quic: Hysteria2, TUIC, DoQ, DoH3;
# dhcp: the "dhcp" DNS server; wireguard: WireGuard nodes; utls: Reality and
# TLS fingerprints; clash_api: the API NextBox drives the core through;
# naive_outbound + purego: NaiveProxy (libcronet.dll); badlinkname and
# tfogo_checklinkname0 belong to the official build flags.
$tags = 'with_gvisor,with_quic,with_dhcp,with_wireguard,with_utls,with_clash_api,with_naive_outbound,with_purego,badlinkname,tfogo_checklinkname0'

# Version from the first changelog heading, e.g. "#### 1.14.4"
$version = (Select-String -Path (Join-Path $Source 'docs\changelog.md') -Pattern '^#### (.+)$' | Select-Object -First 1).Matches[0].Groups[1].Value
$sharedLd = (Get-Content (Join-Path $Source 'release\LDFLAGS') -Raw).Trim()
$ldflags = "-X 'github.com/sagernet/sing-box/constant.Version=$version' $sharedLd -s -w -buildid="

Write-Host "Building sing-box $version"
Write-Host "Tags: $tags"

Push-Location $Source
try {
    $env:CGO_ENABLED = '0'
    $env:GOOS = 'windows'
    $env:GOARCH = 'amd64'
    go build -v -trimpath -tags $tags -ldflags $ldflags -o (Join-Path $Out 'sing-box.exe') ./cmd/sing-box
    if ($LASTEXITCODE -ne 0) { throw "go build failed ($LASTEXITCODE)" }

    # libcronet.dll from the cronet-go version in sing-box's go.mod
    go mod download github.com/sagernet/cronet-go/lib/windows_amd64
    $libDir = (go list -m -f '{{.Dir}}' github.com/sagernet/cronet-go/lib/windows_amd64).Trim()
    if ($LASTEXITCODE -ne 0 -or -not $libDir) { throw 'cronet-go/lib/windows_amd64 not found' }
    Copy-Item -Force (Join-Path $libDir 'libcronet.dll') (Join-Path $Out 'libcronet.dll')
    # module cache files are read-only
    Set-ItemProperty (Join-Path $Out 'libcronet.dll') -Name IsReadOnly -Value $false
}
finally {
    Pop-Location
}
Write-Host "Done: $(Resolve-Path (Join-Path $Out 'sing-box.exe'))"
Write-Host "      $(Resolve-Path (Join-Path $Out 'libcronet.dll'))"
