param([switch]$RebuildBackend, [switch]$ImportModels, [switch]$ImportProductionModels)

$ErrorActionPreference = 'Stop'
$repoDirectory = Split-Path -Parent $PSScriptRoot
Set-Location -LiteralPath $repoDirectory
if ($ImportModels -and $ImportProductionModels) { throw 'Choose only one model import mode.' }

function Test-LocalDockerObject {
    param([string]$Kind, [string]$Name)
    # Windows PowerShell treats native stderr as an error even when redirected.
    # Missing objects are expected here; keep this preference local to the probe.
    $ErrorActionPreference = 'Continue'
    & docker $Kind inspect $Name *> $null
    return $LASTEXITCODE -eq 0
}

if (-not (Get-Command docker -ErrorAction SilentlyContinue)) {
    $dockerDirectories = @(
        (Join-Path $env:ProgramFiles 'Docker/Docker/resources/bin'),
        (Join-Path $env:LOCALAPPDATA 'Programs/DockerDesktop/resources/bin')
    )
    $dockerDirectory = $dockerDirectories | Where-Object {
        Test-Path -LiteralPath (Join-Path $_ 'docker.exe') -ErrorAction SilentlyContinue
    } | Select-Object -First 1
    if (-not $dockerDirectory) { throw 'Install Docker Desktop and start Linux containers.' }
    $env:PATH = "$dockerDirectory;$env:PATH"
}
$bun = 'bun'
if (-not (Get-Command bun -ErrorAction SilentlyContinue)) {
    $bun = Join-Path $env:USERPROFILE '.bun/bin/bun.exe'
    if (-not (Test-Path -LiteralPath $bun)) { throw 'Install Bun, then retry.' }
}
& docker info *> $null
if ($LASTEXITCODE -ne 0) { throw 'Docker is not ready. Start Docker Desktop, then retry.' }

$privateDirectory = Join-Path (Split-Path -Parent $repoDirectory) 'private/shop'
if ($env:RVL_PRIVATE_DIR) { $privateDirectory = $env:RVL_PRIVATE_DIR }
$perfFile = Join-Path $privateDirectory '.env.perf'
# Keep existing installations working while moving credentials outside the checkout.
if (-not (Test-Path -LiteralPath $perfFile -PathType Leaf)) {
    $perfFile = Join-Path $repoDirectory '.env.perf'
}
$perfHash = ''
$perfArguments = @()
if (Test-Path -LiteralPath $perfFile -PathType Leaf) {
    $perfHash = (Get-FileHash -LiteralPath $perfFile -Algorithm SHA256).Hash
    $perfArguments = @('--env-file', $perfFile)
}

if ($RebuildBackend -or -not (Test-LocalDockerObject image 'new-api:local')) {
    Write-Host 'Building the backend only. Frontend changes will use hot reload.'
    & docker build -f Dockerfile.dev -t new-api:local .
    if ($LASTEXITCODE -ne 0) { throw 'Backend build failed. Existing containers were not changed.' }
}

$containerExists = Test-LocalDockerObject container 'new-api-local'
if ($containerExists) {
    $existingImage = & docker container inspect --format '{{.Config.Image}}' new-api-local
    if ($existingImage -ne 'new-api:local') {
        throw 'A different container already uses the name new-api-local. It was not changed.'
    }
    $existingPerfHash = & docker container inspect --format '{{.Config.Labels.rvl_perf_config}}' new-api-local
    if ($LASTEXITCODE -ne 0) { throw 'Could not inspect local performance settings.' }
    if ($existingPerfHash -eq '<no value>') { $existingPerfHash = '' }
}
if ($containerExists -and ($RebuildBackend -or "$existingPerfHash" -ne $perfHash)) {
    & docker rm -f new-api-local
    if ($LASTEXITCODE -ne 0) { throw 'Could not replace the local backend container.' }
    $containerExists = $false
}
if ($containerExists) {
    & docker start new-api-local | Out-Null
} else {
    & docker run -d --name new-api-local --restart unless-stopped `
        -p 127.0.0.1:3001:3000 -v new-api-local-data:/data -e TZ=Asia/Bangkok `
        --label "rvl_perf_config=$perfHash" @perfArguments new-api:local
}
if ($LASTEXITCODE -ne 0) { throw 'Local backend could not start. Check whether port 3001 is occupied.' }

$ready = $false
for ($attempt = 0; $attempt -lt 60; $attempt++) {
    try {
        $response = Invoke-WebRequest -UseBasicParsing -Uri 'http://127.0.0.1:3001/api/status' -TimeoutSec 2
        if ($response.StatusCode -eq 200) { $ready = $true; break }
    } catch { }
    Start-Sleep -Seconds 1
}
if (-not $ready) { throw 'Local backend did not become ready. See: docker logs new-api-local' }

if ($ImportModels -or $ImportProductionModels) {
    $catalog = Join-Path $privateDirectory 'model-catalog.json'
    $importArguments = @('import', '--catalog', '/input/catalog.json')
    if ($ImportProductionModels) {
        $catalog = Join-Path $privateDirectory 'shop-linux-move.sql'
        $importArguments = @('import-shop', '--dump', '/input/catalog.json')
        Write-Host 'Replacing local models, pricing and API channels from the production backup. Customer accounts stay local.'
        Write-Host 'API calls will use the production upstream keys/credits from that backup.'
    }
    if (-not (Test-Path -LiteralPath $catalog -PathType Leaf)) {
        throw "Missing model catalog: $catalog"
    }
    $target = & docker container inspect new-api-local
    if ($LASTEXITCODE -ne 0) { throw 'Could not inspect the local import target.' }
    $target = ($target | ConvertFrom-Json)[0]
    $dataMount = @($target.Mounts | Where-Object { $_.Destination -eq '/data' })
    $externalDatabase = @($target.Config.Env | Where-Object { $_ -match '^SQL_DSN=.+' })
    if ($target.Config.Image -ne 'new-api:local' -or $dataMount.Count -ne 1 -or
        $dataMount[0].Type -ne 'volume' -or $dataMount[0].Name -ne 'new-api-local-data' -or
        $externalDatabase.Count -gt 0) {
        throw 'Refusing model import: target must be the isolated local SQLite volume.'
    }
    & docker pull python:3.12-alpine
    if ($LASTEXITCODE -ne 0) { throw 'Python image download failed. Local backend is still running.' }
    & docker stop new-api-local | Out-Null
    if ($LASTEXITCODE -ne 0) { throw 'Could not pause the local backend for import.' }
    try {
        $importer = Join-Path $PSScriptRoot 'model_catalog.py'
        & docker run --rm --network none --user 0 `
            --mount 'type=volume,source=new-api-local-data,target=/data' `
            --mount "type=bind,source=$catalog,target=/input/catalog.json,readonly" `
            --mount "type=bind,source=$importer,target=/tool/model_catalog.py,readonly" `
            python:3.12-alpine python /tool/model_catalog.py @importArguments --database /data/one-api.db
        if ($LASTEXITCODE -ne 0) { throw 'Model import failed. See the error above.' }
    } finally {
        & docker start new-api-local | Out-Null
        if ($LASTEXITCODE -ne 0) { throw 'Could not restart local backend. Run: docker start new-api-local' }
    }
}

Set-Location -LiteralPath (Join-Path $repoDirectory 'web')
$env:VITE_REACT_APP_SERVER_URL = 'http://127.0.0.1:3001'
& $bun install --frozen-lockfile
if ($LASTEXITCODE -ne 0) { throw 'Frontend dependency installation failed.' }
Write-Host 'Local realtime frontend starts at port 5173; if occupied, it uses the next available port.'
Write-Host 'Backend: 127.0.0.1:3001, isolated local data in new-api-local-data.'
Write-Host 'Save React/CSS files to update the page. No Docker rebuild is needed.'
Write-Host 'For Go changes only: 03-local.bat -RebuildBackend'
Write-Host 'The browser opens the selected URL. Use the Local URL printed by Rsbuild.'
& $bun run dev --host 127.0.0.1 --port 5173 --open
if ($LASTEXITCODE -ne 0) { throw 'Frontend dev server stopped with an error.' }
