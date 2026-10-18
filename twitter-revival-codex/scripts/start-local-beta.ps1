# Start only loopback services and export the public local CA, never its key.
[CmdletBinding()]
param([switch]$Build)
$ErrorActionPreference='Stop'
$projectRoot=[IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))
$dockerCommand=(Get-Command docker -ErrorAction SilentlyContinue).Source
if(!$dockerCommand){$dockerCommand=Join-Path $env:LOCALAPPDATA 'Programs/DockerDesktop/resources/bin/docker.exe'}
if(!(Test-Path -LiteralPath $dockerCommand)){throw 'Avviare/installare Docker Desktop prima di proseguire.'}
$env:PATH=(Split-Path -Parent $dockerCommand)+[IO.Path]::PathSeparator+$env:PATH
& (Join-Path $PSScriptRoot 'prepare-sprint6.ps1')
$compose=@('compose','--project-directory',$projectRoot,'--env-file',(Join-Path $projectRoot '.env'),'-f',(Join-Path $projectRoot 'compose.yaml'),'-f',(Join-Path $projectRoot 'infra/https.compose.yaml'),'--profile','monitoring')
& $dockerCommand @compose config --quiet
if($LASTEXITCODE -ne 0){throw 'Configurazione Compose non valida.'}
$start=@('up','-d','--wait')
if($Build){$start+='--build'}
& $dockerCommand @compose @start
if($LASTEXITCODE -ne 0){throw 'Avvio beta locale non riuscito.'}
& $dockerCommand @compose exec -T gateway caddy validate --config /etc/caddy/Caddyfile
if($LASTEXITCODE -ne 0){throw 'Configurazione HTTPS non valida.'}
& $dockerCommand @compose cp 'gateway:/data/caddy/pki/authorities/local/root.crt' (Join-Path $projectRoot '.local/caddy-root.crt')
if($LASTEXITCODE -ne 0){throw 'Esportazione certificato pubblico CA non riuscita.'}
Write-Output 'Beta: https://localhost:3443 | Monitor: http://localhost:9090 | Casella locale: http://localhost:8025'
Write-Output 'CA pubblica in .local/caddy-root.crt. La fiducia nel browser richiede importazione manuale; nessuna modifica automatica alle autorità di Windows.'
