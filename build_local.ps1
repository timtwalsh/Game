param (
    [switch]$Run = $false
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

Write-Host "Building project workspace..." -ForegroundColor Cyan

Write-Host "Building Server..." -ForegroundColor Cyan
go build -o "$binDir\server.exe" ./server
if ($LASTEXITCODE -ne 0) {
    Write-Host "Server Build failed!" -ForegroundColor Red
    exit $LASTEXITCODE
}

Write-Host "Building Client..." -ForegroundColor Cyan
go build -o "$binDir\client.exe" ./client
if ($LASTEXITCODE -ne 0) {
    Write-Host "Client Build failed!" -ForegroundColor Red
    exit $LASTEXITCODE
}

Write-Host "Build successful! Binaries are located in .\bin\" -ForegroundColor Green

if ($Run) {
    Write-Host "Starting local test environment..." -ForegroundColor Cyan
    
    # Start the server in a new window
    Start-Process -FilePath "$binDir\server.exe" -WindowStyle Normal -PassThru
    
    # Give the server a moment to start up
    Start-Sleep -Seconds 1
    
    # Start the first client
    Start-Process -FilePath "$binDir\client.exe" -WindowStyle Normal -PassThru
    
    Start-Sleep -Milliseconds 500
    
    # Start the second client
    Start-Process -FilePath "$binDir\client.exe" -WindowStyle Normal -PassThru
}
