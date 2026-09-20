# Verifies ScreenState. This is the gate build.ps1 runs before it builds anything.
#
#   ./test.ps1          run every check
#   ./test.ps1 -Quick   skip staticcheck, which fetches a tool on first use
#
# It fails on the first thing that is wrong, so the output ends at the problem
# rather than burying it under everything that came after.
param(
    [switch]$Quick
)

$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $MyInvocation.MyCommand.Path
Set-Location $root

# Cgo is pinned off rather than left to whatever the machine defaults to. Nothing
# here wants a C toolchain; off is already the default on a machine without
# one, which is exactly why leaving it implicit is a trap: the first machine with
# a compiler would quietly produce a different binary and nothing would say so.
$env:CGO_ENABLED = '0'

# The layers a machine can exercise with no filesystem, no clock and no desktop.
# Anything short of whole here is a decision nobody made. Infrastructure is not
# on this list on purpose: the Windows half needs a real desktop; gating it
# would mean either a number that means nothing or tests that assert whatever
# happened to be on screen.
$floored = @(
    'github.com/oernster/ScreenState/internal/domain',
    'github.com/oernster/ScreenState/internal/application'
)
$floor = 100.0

Write-Host 'Checking formatting...'
$unformatted = & gofmt -l .
if ($unformatted) { throw "gofmt would change these files:`n$($unformatted -join "`n")" }

Write-Host 'Vetting...'
& go vet ./...
if ($LASTEXITCODE -ne 0) { throw "go vet failed with exit code $LASTEXITCODE" }

if (-not $Quick) {
    Write-Host 'Running staticcheck...'
    & go run honnef.co/go/tools/cmd/staticcheck@latest ./...
    if ($LASTEXITCODE -ne 0) { throw "staticcheck failed with exit code $LASTEXITCODE" }
}

# The whole product still has to build for a platform that is not Windows. Every
# rule about what a profile means lives in the two portable layers; the day
# that stops being true is the day this check fails rather than the day somebody
# notices.
Write-Host 'Building for a platform that is not Windows...'
$env:GOOS = 'linux'
& go build ./...
$buildResult = $LASTEXITCODE
Remove-Item Env:\GOOS
if ($buildResult -ne 0) { throw "the product no longer builds off Windows (exit code $buildResult)" }

# Two runs, because they cannot be one. The race detector is built on cgo and
# refuses to run without it, while the binary that ships is built with cgo off.
# Running only the race pass would leave the shipping configuration untested;
# running only the other would drop a detector that has already earned its place
# here by finding a race in a test of the restore replacement.
Write-Host 'Running the tests with the race detector...'
$env:CGO_ENABLED = '1'
& go test -race -count=1 ./...
$raceResult = $LASTEXITCODE
$env:CGO_ENABLED = '0'
if ($raceResult -ne 0) { throw "the race pass failed with exit code $raceResult" }

Write-Host 'Running the tests as the product ships, with cgo off...'
$coverage = Join-Path $root 'coverage.out'
& go test -count=1 -covermode=set "-coverprofile=$coverage" ./...
if ($LASTEXITCODE -ne 0) { throw "go test failed with exit code $LASTEXITCODE" }

# The two arguments above are quoted on purpose. An unquoted -flag=$variable is
# handed to the program with the dollar sign still in it, so the coverage output
# landed in a file literally called $coverage. Quoting makes PowerShell expand it
# first. The variable is also deliberately not named $profile, which is the
# automatic variable holding the path of the shell's own profile script.
Write-Host 'Checking the coverage floor...'
$measured = @{}
foreach ($line in (& go tool cover "-func=$coverage")) {
    if ($line -match '^total:') { continue }
    if ($line -notmatch '^(\S+\.go):\d+:\s+\S+\s+([\d.]+)%$') { continue }
    $package = Split-Path -Parent $Matches[1]
    $package = $package -replace '\\', '/'
    if (-not $measured.ContainsKey($package)) { $measured[$package] = @() }
    $measured[$package] += [double]$Matches[2]
}
foreach ($package in $floored) {
    if (-not $measured.ContainsKey($package)) { throw "no coverage was measured for $package" }
    $statements = $measured[$package]
    $covered = ($statements | Where-Object { $_ -gt 0 }).Count
    $percent = [math]::Round(100.0 * $covered / $statements.Count, 1)
    if ($percent -lt $floor) { throw "$package is at $percent%, below the floor of $floor%" }
    Write-Host "  $package $percent%"
}

Remove-Item $coverage -Force -ErrorAction SilentlyContinue
Write-Host 'All checks passed.'
