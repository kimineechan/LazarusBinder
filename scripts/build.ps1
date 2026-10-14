$ErrorActionPreference = 'Stop'
Set-Location (Join-Path $PSScriptRoot '..')
if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
    throw 'Install Go from https://go.dev/dl/, reopen PowerShell, then run this script.'
}
$env:GOOS = 'windows'
$env:GOARCH = 'amd64'
$env:CGO_ENABLED = '0'
New-Item -ItemType Directory -Force dist | Out-Null
Copy-Item assets/launcher-normal.syso cmd/launcher/resources_windows_amd64.syso -Force
go build -buildvcs=false -trimpath -ldflags '-H=windowsgui' -o cmd/lazarusbinder/launcher.exe ./cmd/launcher
if ($LASTEXITCODE -ne 0) { throw 'Launcher build failed.' }
try {
    Copy-Item assets/launcher-admin.syso cmd/launcher/resources_windows_amd64.syso -Force
    go build -buildvcs=false -trimpath -ldflags '-H=windowsgui' -o cmd/lazarusbinder/launcher-admin.exe ./cmd/launcher
    if ($LASTEXITCODE -ne 0) { throw 'Administrator launcher build failed.' }
} finally {
    Copy-Item assets/launcher-normal.syso cmd/launcher/resources_windows_amd64.syso -Force
}
$env:LAZARUSBINDER_TEST_EXE = (Resolve-Path cmd/lazarusbinder/launcher.exe).Path
$env:LAZARUSBINDER_TEST_ADMIN_EXE = (Resolve-Path cmd/lazarusbinder/launcher-admin.exe).Path
go test -buildvcs=false ./internal/...
if ($LASTEXITCODE -ne 0) { throw 'Tests failed.' }
go build -buildvcs=false -trimpath -ldflags '-H=windowsgui' -o dist/LazarusBinder.exe ./cmd/lazarusbinder
if ($LASTEXITCODE -ne 0) { throw 'LazarusBinder build failed.' }
go vet -buildvcs=false ./cmd/lazarusbinder ./cmd/launcher ./internal/...
if ($LASTEXITCODE -ne 0) { throw 'Go vet failed.' }
go run -buildvcs=false ./cmd/verify
if ($LASTEXITCODE -ne 0) { throw 'Release verification failed.' }
Write-Host 'Built dist\LazarusBinder.exe'
