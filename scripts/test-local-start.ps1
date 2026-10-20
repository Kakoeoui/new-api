$ErrorActionPreference = 'Stop'
$runner = Join-Path $PSScriptRoot 'start-local.ps1'
$originalLocation = Get-Location
$originalServerUrl = $env:VITE_REACT_APP_SERVER_URL
$global:localDockerCalls = New-Object 'System.Collections.Generic.List[string]'
$global:localBunCalls = New-Object 'System.Collections.Generic.List[string]'
$global:localBuildFails = $false
$global:localBackendReady = $true
$global:localImageMissing = $false
$global:localContainerMissing = $false
$global:localImportFails = $false
$global:localTargetVolume = 'new-api-local-data'
$global:localPerfFile = $false
$global:localPerfHash = ''

function global:Test-Path {
    param([string]$LiteralPath, [string]$PathType)
    if ($LiteralPath -like '*.env.perf') { return $global:localPerfFile }
    if ($PathType) { return Microsoft.PowerShell.Management\Test-Path -LiteralPath $LiteralPath -PathType $PathType }
    return Microsoft.PowerShell.Management\Test-Path -LiteralPath $LiteralPath
}
function global:Get-FileHash { return [pscustomobject]@{Hash = 'TEST_PERF_HASH'} }

function global:docker {
    $call = $args -join ' '
    $global:localDockerCalls.Add($call)
    $global:LASTEXITCODE = 0
    if (($args[0] -eq 'image' -and $args[1] -eq 'inspect' -and $global:localImageMissing) -or
        ($args[0] -eq 'container' -and $args[1] -eq 'inspect' -and $global:localContainerMissing)) {
        # Use a real native stderr/exit code to reproduce Windows PowerShell 5.1.
        & $env:ComSpec /d /c 'echo Error response from daemon: No such image or container 1>&2 & exit /b 1'
        $global:LASTEXITCODE = 1
        return
    }
    if ($call -like '*{{.Config.Image}}*') { return 'new-api:local' }
    if ($call -like '*{{.Config.Labels.rvl_perf_config}}*') { return $global:localPerfHash }
    if ($call -eq 'container inspect new-api-local') {
        return (ConvertTo-Json -Depth 5 -InputObject @(@{
            Config = @{ Image = 'new-api:local'; Env = @('TZ=Asia/Bangkok') }
            Mounts = @(@{ Destination = '/data'; Type = 'volume'; Name = $global:localTargetVolume })
        }))
    }
    if ($args[0] -eq 'run' -and $call -like '*model_catalog.py*' -and $global:localImportFails) {
        $global:LASTEXITCODE = 1
        return
    }
    if ($global:localBuildFails -and $args[0] -eq 'build') { $global:LASTEXITCODE = 1 }
    if (-not $global:localBuildFails -and $args[0] -eq 'build') { $global:localImageMissing = $false }
    if ($args[0] -eq 'run') { $global:localContainerMissing = $false }
}
function global:bun {
    $global:localBunCalls.Add(($args -join ' '))
    $global:LASTEXITCODE = 0
}
function global:Invoke-WebRequest {
    if (-not $global:localBackendReady) { throw 'Synthetic backend unavailable' }
    return [pscustomobject]@{StatusCode = 200}
}
function global:Start-Sleep { }

try {
    $env:VITE_REACT_APP_SERVER_URL = 'http://127.0.0.1:18474'
    & $runner
    if ($localDockerCalls | Where-Object { $_ -match '^build ' }) { throw 'Warm start rebuilt the backend.' }
    if ($env:VITE_REACT_APP_SERVER_URL -ne 'http://127.0.0.1:3001') { throw 'Local frontend used customer data.' }
    if ($localBunCalls[0] -ne 'install --frozen-lockfile') { throw 'Missing frozen dependency install.' }
    if ($localBunCalls[1] -ne 'run dev --host 127.0.0.1 --port 5173 --open') { throw 'Local hot reload must allow Rsbuild to select a free port.' }

    $localDockerCalls.Clear()
    $localBunCalls.Clear()
    $global:localImageMissing = $true
    $global:localContainerMissing = $true
    & $runner
    if (@($localDockerCalls | Where-Object { $_ -match '^build ' }).Count -ne 1) { throw 'Cold start must build the missing backend image once.' }
    if (-not ($localDockerCalls | Where-Object { $_ -match '^run .*new-api-local-data:/data' })) { throw 'Cold start did not create the isolated local backend.' }
    if ($localDockerCalls | Where-Object { $_ -match '^rm ' }) { throw 'Cold start removed an existing container.' }
    if ($localBunCalls.Count -ne 2) { throw 'Cold start did not reach the realtime frontend.' }

    $localDockerCalls.Clear()
    $global:localPerfFile = $true
    & $runner
    if (-not ($localDockerCalls | Where-Object { $_ -match '^run .*--env-file .*\.env.perf' })) { throw 'Missing upstream environment file.' }
    if ($localDockerCalls | Where-Object { $_ -match '^build ' }) { throw 'Performance config changes must not rebuild the image.' }
    $global:localPerfHash = 'TEST_PERF_HASH'
    $localDockerCalls.Clear()
    & $runner
    if ($localDockerCalls | Where-Object { $_ -match '^(rm|run) ' }) { throw 'Unchanged performance settings replaced the container.' }
    $global:localPerfFile = $false
    $global:localPerfHash = ''

    $localDockerCalls.Clear()
    $localBunCalls.Clear()
    $global:localBuildFails = $true
    $caught = $false
    try { & $runner -RebuildBackend } catch { $caught = $true }
    if (-not $caught) { throw 'A backend build failure was ignored.' }
    if ($localDockerCalls | Where-Object { $_ -match '^(rm|run|start) ' }) { throw 'Build failure changed a container.' }
    if ($localBunCalls.Count -ne 0) { throw 'Build failure started the frontend.' }

    $global:localBuildFails = $false
    $localDockerCalls.Clear()
    & $runner -ImportModels
    if (-not ($localDockerCalls | Where-Object { $_ -match '^run .*--network none .*source=new-api-local-data.*model_catalog.py import' })) {
        throw 'Import must use the isolated local volume with no network.'
    }
    if ($localDockerCalls[-1] -ne 'start new-api-local') { throw 'Import did not restart local backend.' }

    $localDockerCalls.Clear()
    $global:localImportFails = $true
    $caught = $false
    try { & $runner -ImportModels } catch { $caught = $true }
    if (-not $caught -or $localDockerCalls[-1] -ne 'start new-api-local') { throw 'Import failure did not restart backend and report failure.' }
    $global:localImportFails = $false

    $localDockerCalls.Clear()
    & $runner -ImportProductionModels
    if (-not ($localDockerCalls | Where-Object { $_ -match '^run .*source=new-api-local-data.*model_catalog.py import-shop --dump ' })) {
        throw 'Full model import did not use the local volume and scoped shop importer.'
    }

    $localDockerCalls.Clear()
    $global:localTargetVolume = 'new-api-customer-postgres'
    $caught = $false
    try { & $runner -ImportModels } catch { $caught = $true }
    if (-not $caught -or ($localDockerCalls | Where-Object { $_ -match '^(stop|pull|run) ' })) { throw 'Wrong import target was not refused before mutation.' }
    $global:localTargetVolume = 'new-api-local-data'

    $global:localBackendReady = $false
    $localBunCalls.Clear()
    $caught = $false
    try { & $runner } catch { $caught = $true }
    if (-not $caught) { throw 'Backend readiness failure was ignored.' }
    if ($localBunCalls.Count -ne 0) { throw 'Frontend started without its local backend.' }
    Write-Host 'PASS: missing image/container native stderr triggers cold-start build/run, warm start skips builds, local data is isolated, HMR is used, failed builds/readiness stop frontend startup. Docker/Bun/HTTP were mocked.'
} finally {
    Set-Location $originalLocation
    $env:VITE_REACT_APP_SERVER_URL = $originalServerUrl
    foreach ($functionName in @('docker', 'bun', 'Invoke-WebRequest', 'Start-Sleep', 'Test-Path', 'Get-FileHash')) {
        Remove-Item "Function:\$functionName"
    }
}
