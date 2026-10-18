# Local-only backup and isolated restore rehearsal. Requires running Compose
# services and the Sprint 5 API image. Never restores over the active database.
[CmdletBinding()]
param()
$ErrorActionPreference = 'Stop'
$projectRoot = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))
$backupRoot = [IO.Path]::GetFullPath((Join-Path $projectRoot '.local/backups/sprint5'))
$restoreFile = Join-Path $projectRoot 'infra/restore.compose.yaml'
$envFile = Join-Path $projectRoot '.env'
$dockerCommand = (Get-Command docker -ErrorAction SilentlyContinue).Source
if (!$dockerCommand) {
    $dockerCommand = Join-Path $env:LOCALAPPDATA 'Programs/DockerDesktop/resources/bin/docker.exe'
}
if (!(Test-Path -LiteralPath $dockerCommand)) { throw 'Docker Desktop non trovato.' }
if (!(Test-Path -LiteralPath $envFile)) { throw 'Configurare .env prima del backup.' }

function Invoke-Docker([string[]]$Arguments) {
    $result = & $dockerCommand @Arguments
    if ($LASTEXITCODE -ne 0) { throw 'Comando Docker fallito; backup non verificato.' }
    return $result
}
function Get-CheckedBackupPath([string]$Path) {
    $resolved = [IO.Path]::GetFullPath($Path)
    if (!$resolved.StartsWith($backupRoot + [IO.Path]::DirectorySeparatorChar, [StringComparison]::OrdinalIgnoreCase)) { throw 'Percorso fuori dalla cartella backup prevista.' }
    return $resolved
}
$short = [guid]::NewGuid().ToString('N').Substring(0,8)
$stamp = [DateTime]::UtcNow.ToString('yyyyMMddTHHmmssZ') + '_' + $short
$backupPath = Get-CheckedBackupPath (Join-Path $backupRoot $stamp)
[void](New-Item -ItemType Directory -Path $backupPath -Force)
if ([Environment]::OSVersion.Platform -eq [PlatformID]::Win32NT) {
    $identity = [Security.Principal.WindowsIdentity]::GetCurrent().Name
    & "$env:SystemRoot/System32/icacls.exe" $backupRoot '/inheritance:r' '/grant:r' "${identity}:(OI)(CI)F" | Out-Null
    if ($LASTEXITCODE -ne 0) { throw 'Impossibile proteggere i backup con ACL locali.' }
}
$source = @('compose','--project-directory',$projectRoot,'--env-file',$envFile,'-f',(Join-Path $projectRoot 'compose.yaml'))
$restoreProject = 'radura-restore-' + $short
if ($restoreProject -notmatch '^radura-restore-[a-f0-9]{8}$') { throw 'Nome restore non valido.' }
$restore = @('compose','--project-directory',$projectRoot,'--env-file',$envFile,'-p',$restoreProject,'-f',$restoreFile)
$previousImage = $env:RADURA_RESTORE_API_IMAGE
$restoreCreated = $false
try {
    $apiContainer = (Invoke-Docker ($source + @('ps','-q','api')) | Out-String).Trim()
    $storageContainer = (Invoke-Docker ($source + @('ps','-q','storage')) | Out-String).Trim()
    if (!$apiContainer -or !$storageContainer) { throw 'Avviare API e storage prima del backup.' }
    $env:RADURA_RESTORE_API_IMAGE = (Invoke-Docker @('inspect','--format','{{.Image}}',$apiContainer) | Out-String).Trim()
    $mounts = (Invoke-Docker @('inspect','--format','{{json .Mounts}}',$storageContainer) | Out-String | ConvertFrom-Json)
    $storageVolume = ($mounts | Where-Object { $_.Destination -eq '/data' -and $_.Type -eq 'volume' }).Name
    if (!$storageVolume) { throw 'Volume S3 locale non trovato.' }
    Write-Host 'Pausa delle scritture, snapshot database e storage...'
    try {
        [void](Invoke-Docker ($source + @('stop','api')))
        $manifest = (Invoke-Docker ($source + @('run','--rm','--no-deps','api','-backup-manifest')) | Out-String).Trim()
        [void]($manifest | ConvertFrom-Json)
        [IO.File]::WriteAllText((Join-Path $backupPath 'manifest.json'),$manifest)
        [void](Invoke-Docker ($source + @('exec','-T','db','sh','-c','pg_dump -U "$POSTGRES_USER" -d "$POSTGRES_DB" -Fc --no-owner -f /tmp/radura-backup.dump')))
        [void](Invoke-Docker ($source + @('cp','db:/tmp/radura-backup.dump',(Join-Path $backupPath 'database.dump'))))
        [void](Invoke-Docker ($source + @('stop','storage')))
        [void](Invoke-Docker @('run','--rm','--network','none','--entrypoint','tar','-v',"${storageVolume}:/source:ro",'-v',"${backupPath}:/backup",'postgres:18-alpine','-cf','/backup/media.tar','-C','/source','.'))
    } finally {
        [void](Invoke-Docker ($source + @('up','-d','--wait','storage')))
        [void](Invoke-Docker ($source + @('start','api')))
    }
    Write-Host 'Ripristino in database e volume temporanei isolati...'
    $restoreCreated = $true
    [void](Invoke-Docker ($restore + @('create','db','storage')))
    $restoreStorage = (Invoke-Docker ($restore + @('ps','--all','-q','storage')) | Out-String).Trim()
    $restoreMounts = (Invoke-Docker @('inspect','--format','{{json .Mounts}}',$restoreStorage) | Out-String | ConvertFrom-Json)
    $restoreVolume = ($restoreMounts | Where-Object { $_.Destination -eq '/data' }).Name
    if ($restoreVolume -ne ($restoreProject + '_restore_media') -or $restoreVolume -eq $storageVolume) { throw 'Volume di restore non isolato.' }
    [void](Invoke-Docker @('run','--rm','--network','none','--entrypoint','tar','-v',"${restoreVolume}:/destination",'-v',"${backupPath}:/backup:ro",'postgres:18-alpine','-xf','/backup/media.tar','-C','/destination'))
    [void](Invoke-Docker ($restore + @('up','-d','--wait','db','storage')))
    [void](Invoke-Docker ($restore + @('cp',(Join-Path $backupPath 'database.dump'),'db:/tmp/restore.dump')))
    [void](Invoke-Docker ($restore + @('exec','-T','db','sh','-c','pg_restore -U "$POSTGRES_USER" -d "$POSTGRES_DB" --exit-on-error --no-owner /tmp/restore.dump')))
    $restored = (Invoke-Docker ($restore + @('run','--rm','--no-deps','verify','-backup-manifest')) | Out-String).Trim()
    $expected = $manifest | ConvertFrom-Json | ConvertTo-Json -Depth 50 -Compress
    $actual = $restored | ConvertFrom-Json | ConvertTo-Json -Depth 50 -Compress
    if ($expected -cne $actual) { throw 'Digest dei dati o delle immagini diverso dopo restore.' }
    $verified = $manifest | ConvertFrom-Json
    $receipt = [ordered]@{ verified_at=[DateTime]::UtcNow.ToString('o'); tables=$verified.tables.Count; rows=($verified.tables | Measure-Object -Property rows -Sum).Sum; objects=$verified.objects.Count; database_sha256=(Get-FileHash -LiteralPath (Join-Path $backupPath 'database.dump') -Algorithm SHA256).Hash; media_sha256=(Get-FileHash -LiteralPath (Join-Path $backupPath 'media.tar') -Algorithm SHA256).Hash }
    $receipt | ConvertTo-Json | Set-Content -LiteralPath (Join-Path $backupPath 'verified.json') -Encoding UTF8
    # Prune only this script's own dated folders. Never touch older manual dumps.
    Get-ChildItem -LiteralPath $backupRoot -Directory | Where-Object { $_.Name -match '^\d{8}T\d{6}Z_[a-f0-9]{8}$' -and $_.CreationTimeUtc -lt [DateTime]::UtcNow.AddDays(-30) } | ForEach-Object {
        $checked = Get-CheckedBackupPath $_.FullName
        if ((Get-Item -LiteralPath $checked).Attributes -band [IO.FileAttributes]::ReparsePoint) { throw 'Link inatteso nella cartella backup.' }
        $entries = @(Get-ChildItem -LiteralPath $checked -Force)
        foreach ($entry in $entries) {
            if ($entry.PSIsContainer -or ($entry.Attributes -band [IO.FileAttributes]::ReparsePoint) -or $entry.Name -notin @('manifest.json','database.dump','media.tar','verified.json')) { throw 'Contenuto inatteso nel backup scaduto: pulizia interrotta.' }
        }
        foreach ($entry in $entries) { Remove-Item -LiteralPath (Get-CheckedBackupPath $entry.FullName) -Force }
        Remove-Item -LiteralPath $checked -Force
    }
    Write-Host "Backup e restore verificati: $backupPath"
    Write-Host "Tabelle: $($receipt.tables); righe: $($receipt.rows); immagini S3: $($receipt.objects)."
} finally {
    if ($restoreCreated) { [void](Invoke-Docker ($restore + @('down','--volumes','--remove-orphans'))) }
    $env:RADURA_RESTORE_API_IMAGE = $previousImage
}
