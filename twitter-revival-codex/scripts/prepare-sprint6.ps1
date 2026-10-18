# Add only missing local configuration; never rotate existing DB/S3 credentials.
$ErrorActionPreference='Stop'
$root=[IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))
$envPath=Join-Path $root '.env'
if(!(Test-Path -LiteralPath $envPath)){throw 'Configurare .env dal README prima di proseguire.'}
$content=[IO.File]::ReadAllText($envPath)
$matchesFound=@($content -split '\r?\n'|Where-Object {$_ -match '^METRICS_TOKEN='})
if($matchesFound.Count -gt 1){throw 'METRICS_TOKEN duplicato in .env.'}
if(!$matchesFound.Count){
 $bytes=New-Object byte[] 32
 $generator=[Security.Cryptography.RandomNumberGenerator]::Create()
 try{$generator.GetBytes($bytes)}finally{$generator.Dispose()}
 $token=[BitConverter]::ToString($bytes).Replace('-','').ToLowerInvariant()
 [IO.File]::AppendAllText($envPath,"`nMETRICS_TOKEN=$token`n",[Text.UTF8Encoding]::new($false))
}else{$token=$matchesFound[0].Substring('METRICS_TOKEN='.Length)}
if($token.Length -lt 32 -or $token -match '[\r\n]'){throw 'METRICS_TOKEN deve contenere almeno 32 caratteri.'}
[void](New-Item -ItemType Directory -Force -Path (Join-Path $root '.local'))
[IO.File]::WriteAllText((Join-Path $root '.local/metrics-token'),$token,[Text.UTF8Encoding]::new($false))
Write-Output 'Configurazione monitor locale pronta; segreti esistenti conservati.'
