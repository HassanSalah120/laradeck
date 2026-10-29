param([string]$GoExecutable = "go", [switch]$SkipTests, [string]$OutputFilename = "laradeck.exe")
$ErrorActionPreference = "Stop"
$originalBuildPath = $env:PATH
Push-Location $PSScriptRoot
try {
    if (!(Get-Command $GoExecutable -ErrorAction SilentlyContinue)) {
        $bundledGo = Join-Path $env:USERPROFILE '.codex\tools\go\bin\go.exe'
        if (!(Test-Path -LiteralPath $bundledGo)) { throw 'Install Go and add it to PATH, or pass -GoExecutable.' }
        $GoExecutable = $bundledGo
    }
    if ([IO.Path]::GetFileName($OutputFilename) -ne $OutputFilename -or [IO.Path]::GetExtension($OutputFilename) -ne '.exe') { throw 'OutputFilename must be an .exe filename, without a directory.' }
    $targetExecutable = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot "build/bin/$OutputFilename"))
    if (Get-Process | Where-Object { $_.Path -eq $targetExecutable }) { throw "Close $OutputFilename before rebuilding it, or pass -OutputFilename with another filename." }
    $resolvedGoCommand = (Get-Command $GoExecutable -ErrorAction Stop).Source
    $env:PATH = (Split-Path $resolvedGoCommand -Parent) + ';' + $env:PATH
    if (!$SkipTests) {
        & $GoExecutable test ./internal/... -count=1
        if ($LASTEXITCODE -ne 0) { throw 'Go tests failed.' }
        & $GoExecutable vet ./...
        if ($LASTEXITCODE -ne 0) { throw 'Go vet failed.' }
        Push-Location (Join-Path $PSScriptRoot 'frontend')
        try {
            $npmCmd = if (Get-Command "npm.cmd" -ErrorAction SilentlyContinue) { "npm.cmd" } else { "npm" }
            & $npmCmd test
            if ($LASTEXITCODE -ne 0) { throw 'Frontend tests failed.' }
        } finally { Pop-Location }
    }
    & $GoExecutable run github.com/wailsapp/wails/v2/cmd/wails@v2.15.0 build -o $OutputFilename
    if ($LASTEXITCODE -ne 0) { throw 'Wails build failed.' }
    # Keep the console helper beside the chosen desktop executable.
    & $GoExecutable build -trimpath -ldflags '-s -w' -o build/bin/laradeck-runner.exe ./cmd/runner
    & $GoExecutable build -trimpath -ldflags '-s -w' -o build/bin/workbench-runner.exe ./cmd/runner
    if ($LASTEXITCODE -ne 0) { throw 'Process helper build failed.' }
    Write-Output "Ready: build/bin/$OutputFilename (keep laradeck-runner.exe beside it)."
} finally { $env:PATH = $originalBuildPath; Pop-Location }
