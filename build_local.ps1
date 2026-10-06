param (
    [switch]$NoRun = $false,
    # Folder holding world/ and levels/. The server and both clients are all
    # given the same one: predicting and validating against different maps
    # would flag honest players. Without those folders the game runs on a
    # blank grid.
    [string]$Root = $PSScriptRoot
)

# Absolute and without a trailing slash: the processes start in the repo
# root, not the caller's folder, and a trailing backslash would escape the
# closing quote around the -root argument.
if (Test-Path $Root) {
    $Root = (Resolve-Path $Root).Path
}
$Root = $Root.TrimEnd('\', '/')

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

# blobtemplate is pure Go in the game module, so it builds whenever the game
# does. It's a generator for tile templates and placeholder art (see
# docs/LEVEL_MAKER_SPEC.md step 2), run by hand when needed, so it isn't
# launched below.
# The level maker is in the game module and uses the same raylib as the
# client, so it builds whenever the client does.
Write-Host "Building Level maker..." -ForegroundColor Cyan
go build -o "$binDir\levelmaker.exe" ./cmd/levelmaker
if ($LASTEXITCODE -ne 0) {
    Write-Host "Level maker build failed!" -ForegroundColor Red
    exit $LASTEXITCODE
}

Write-Host "Building blobtemplate..." -ForegroundColor Cyan
go build -o "$binDir\blobtemplate.exe" ./cmd/blobtemplate
if ($LASTEXITCODE -ne 0) {
    Write-Host "blobtemplate build failed!" -ForegroundColor Red
    exit $LASTEXITCODE
}

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

    $worldDir = Join-Path $Root "world"
    $levelsDir = Join-Path $Root "levels"
    if ((Test-Path $worldDir) -and (Test-Path $levelsDir)) {
        Write-Host "World: $Root" -ForegroundColor Cyan
    } else {
        Write-Host "No world\ and levels\ under $Root - the game will run on a blank grid." -ForegroundColor Yellow
    }

    # Start the server
    $server = Start-Process -FilePath "$binDir\server.exe" -ArgumentList "-root", "`"$Root`"" -WorkingDirectory $PSScriptRoot -WindowStyle Normal -PassThru
    $null = $server.Handle # Windows PowerShell only reports ExitCode if the handle was opened before exit

    # Give the server a moment to start up. A world that's present but
    # broken makes it exit straight away (its window closes before it can
    # be read), so check for that rather than starting clients against
    # nothing.
    Start-Sleep -Seconds 1
    if ($server.HasExited) {
        Write-Host "The server exited at startup (code $($server.ExitCode)). Usually that's a broken world - run '.\bin\server.exe -root `"$Root`"' in a terminal to see why, or 'go test ./shared/world' to check the sample levels." -ForegroundColor Red
        exit 1
    }

    # Start two clients, so you can see multiplayer sync locally
    Start-Process -FilePath "$binDir\client.exe" -ArgumentList "-root", "`"$Root`"" -WorkingDirectory $PSScriptRoot -WindowStyle Normal -PassThru
    Start-Sleep -Milliseconds 500
    Start-Process -FilePath "$binDir\client.exe" -ArgumentList "-root", "`"$Root`"" -WorkingDirectory $PSScriptRoot -WindowStyle Normal -PassThru

    # Start every tool that built successfully. The level maker edits the
    # same world the game just loaded; restart the server and clients to
    # play what it saves.
    Start-Sleep -Milliseconds 500
    Start-Process -FilePath "$binDir\levelmaker.exe" -ArgumentList "-root", "`"$Root`"" -WorkingDirectory $PSScriptRoot -WindowStyle Normal -PassThru
    if ($animakerOk) {
        Start-Sleep -Milliseconds 500
        Start-Process -FilePath "$binDir\animaker.exe" -WorkingDirectory (Join-Path $PSScriptRoot "animaker") -WindowStyle Normal -PassThru
    }
}

# The game (server + client) building is what determines success; an
# animaker build failure is already reported above as a warning, not a
# script failure - always report overall success once we reach this point.
exit 0
