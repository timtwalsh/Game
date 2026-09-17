param (
    [switch]$NoRun = $false
)

Write-Host "Checking for Go..." -ForegroundColor Cyan
if (!(Get-Command "go" -ErrorAction SilentlyContinue)) {
    Write-Host "Error: 'go' is not installed or not in your PATH." -ForegroundColor Red
    Write-Host "Please install Golang from https://go.dev/ and restart your terminal." -ForegroundColor Yellow
    exit 1
}

# Ensure bin directory exists
$binDir = Join-Path $PSScriptRoot "bin"
if (!(Test-Path $binDir)) {
    New-Item -ItemType Directory -Path $binDir | Out-Null
}

Write-Host "Building game (module: game)..." -ForegroundColor Cyan

Write-Host "Building Server..." -ForegroundColor Cyan
go build -o "$binDir\server.exe" ./server
if ($LASTEXITCODE -ne 0) {
    Write-Host "Server build failed!" -ForegroundColor Red
    exit $LASTEXITCODE
}

Write-Host "Building Client..." -ForegroundColor Cyan
go build -o "$binDir\client.exe" ./client
if ($LASTEXITCODE -ne 0) {
    Write-Host "Client build failed!" -ForegroundColor Red
    exit $LASTEXITCODE
}

Write-Host "Building tools..." -ForegroundColor Cyan

# Animaker (Fyne) needs cgo, which needs a real C compiler. Find one even
# if it's not on PATH, rather than failing outright - this machine has
# WinLibs GCC installed via winget but not exposed on PATH by default.
$cgoEnv = @{}
if (!(Get-Command "gcc" -ErrorAction SilentlyContinue)) {
    $winlibsGcc = Get-ChildItem "$env:LOCALAPPDATA\Microsoft\WinGet\Packages\BrechtSanders.WinLibs*\mingw64\bin\gcc.exe" -ErrorAction SilentlyContinue | Select-Object -First 1
    if ($winlibsGcc) {
        $cgoEnv["PATH"] = "$($winlibsGcc.DirectoryName);$env:PATH"
    }
}

Write-Host "Building Animaker..." -ForegroundColor Cyan
$animakerOk = $true
$oldPath = $env:PATH
$oldCgoEnabled = $env:CGO_ENABLED
Push-Location (Join-Path $PSScriptRoot "animaker")
try {
    if ($cgoEnv.ContainsKey("PATH")) {
        $env:PATH = $cgoEnv["PATH"]
    }
    $env:CGO_ENABLED = "1"
    go build -o "$binDir\animaker.exe" .
    if ($LASTEXITCODE -ne 0) {
        $animakerOk = $false
    }
} finally {
    $env:PATH = $oldPath
    $env:CGO_ENABLED = $oldCgoEnabled
    Pop-Location
}
if (-not $animakerOk) {
    Write-Host "Animaker build failed - skipping it. Needs a C compiler (cgo) on PATH; see map/effects/CONTEXT.md for the current known-good setup." -ForegroundColor Yellow
}

Write-Host "Build complete! Binaries are located in .\bin\" -ForegroundColor Green

if (-not $NoRun) {
    Write-Host "Starting local test environment..." -ForegroundColor Cyan

    # Start the server
    Start-Process -FilePath "$binDir\server.exe" -WorkingDirectory $PSScriptRoot -WindowStyle Normal -PassThru

    # Give the server a moment to start up
    Start-Sleep -Seconds 1

    # Start two clients, so you can see multiplayer sync locally
    Start-Process -FilePath "$binDir\client.exe" -WorkingDirectory $PSScriptRoot -WindowStyle Normal -PassThru
    Start-Sleep -Milliseconds 500
    Start-Process -FilePath "$binDir\client.exe" -WorkingDirectory $PSScriptRoot -WindowStyle Normal -PassThru

    # Start every tool that built successfully
    if ($animakerOk) {
        Start-Sleep -Milliseconds 500
        Start-Process -FilePath "$binDir\animaker.exe" -WorkingDirectory (Join-Path $PSScriptRoot "animaker") -WindowStyle Normal -PassThru
    }
}

# The game (server + client) building is what determines success; an
# animaker build failure is already reported above as a warning, not a
# script failure - always report overall success once we reach this point.
exit 0
