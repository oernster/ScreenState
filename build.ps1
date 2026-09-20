# Builds ScreenState: the application executable; the setup program once one
# exists.
#
#   ./build.ps1                 build everything there is to build
#   ./build.ps1 -SkipInstaller  build only the application
#
# Outputs:
#   build/bin/ScreenState.exe   the agent
#
# The version is read from VERSION and passed in with -ldflags, so no version
# literal lives anywhere in the source. Note that -X only reaches a var: against
# a const it silently does nothing, which is a whole release shipped announcing
# 0.0.0-dev, so main.version is declared a var on purpose.
param(
    [switch]$SkipInstaller
)

$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $MyInvocation.MyCommand.Path
Set-Location $root

$version = (Get-Content (Join-Path $root 'VERSION')).Trim()
Write-Host "Building ScreenState $version"

# Set before the gate as well as the build, so the tests exercise the same
# configuration that ships.
$env:CGO_ENABLED = '0'

# Verify before building, with no way past it. A gate that can be skipped is a
# gate that is skipped on the day it would have caught something, so there is no
# switch to turn this off: run test.ps1 directly while working; let the build
# insist.
& (Join-Path $root 'test.ps1')
if ($LASTEXITCODE -ne 0) { throw "test.ps1 failed with exit code $LASTEXITCODE" }

# The icon and the version Windows shows in the file's properties are carried by
# a resource object the Go toolchain links in when it finds one beside the main
# package. goversioninfo writes it from versioninfo.json.
#
# This is a warning rather than a failure. A build with the default icon is an
# ordinary thing to want while the artwork is still being made; refusing to
# build over it would only teach whoever hits it to comment this section out.
$syso = Join-Path $root 'resource_windows.syso'
$icon = Join-Path $root 'assets/application-icon.ico'
$manifest = Join-Path $root 'versioninfo.json'
if ((Test-Path $icon) -and (Test-Path $manifest)) {
    Write-Host 'Applying the icon and the version resource...'
    & go run github.com/josephspurrier/goversioninfo/cmd/goversioninfo@latest -o $syso -icon $icon -product-version $version -ver-major ($version.Split('.')[0]) -ver-minor ($version.Split('.')[1]) -ver-patch ($version.Split('.')[2]) $manifest
    if ($LASTEXITCODE -ne 0) { throw "goversioninfo failed with exit code $LASTEXITCODE" }
} else {
    Write-Warning 'No assets/application-icon.ico or versioninfo.json: building with the default icon and no version properties.'
    if (Test-Path $syso) { Remove-Item $syso -Force }
}

Write-Host 'Building the application...'
$binDir = Join-Path $root 'build/bin'
New-Item -ItemType Directory -Force -Path $binDir | Out-Null
$exe = Join-Path $binDir 'ScreenState.exe'

# -H=windowsgui gives the agent no console window, which is what a program that
# lives in the tray wants: started at sign-in it must not flash a black box;
# started from a prompt it must not hold the prompt. It also means the run has no
# error output of its own, which is precisely why the log takes it over.
& go build -trimpath -ldflags "-s -w -H=windowsgui -X main.version=$version" -o $exe .
if ($LASTEXITCODE -ne 0) { throw "go build failed with exit code $LASTEXITCODE" }

# The resource object is a build output rather than source. Leaving it behind
# makes `go build ./...` silently link it into every later build, including ones
# meant to carry a different version.
if (Test-Path $syso) { Remove-Item $syso -Force }

Write-Host "Built $exe"

if ($SkipInstaller) {
    exit 0
}

# The setup program is not written yet. It is an application of its own rather
# than a step of this script: a screen stack covering install, update, repair and
# uninstall, writing everything per user so Windows never asks for administrator
# rights; registering itself in the Apps list. When it exists it goes in
# installer/ and is built here, with the application zipped as its payload.
#
# Said out loud rather than passed over, so that a build which produced only the
# executable is never mistaken for a build that produced everything.
Write-Warning 'The setup program is not built yet: this run produced the application only.'
