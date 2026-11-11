# Name-de: Downloads älter als 90 Tage
# Name-en: Downloads older than 90 days
# Description-de: Löscht Dateien im Download-Ordner, die seit 90 Tagen nicht geändert wurden.
# Description-en: Deletes files in the Downloads folder not modified for 90 days.
# Preview: true

$dir   = Join-Path $env:USERPROFILE 'Downloads'
$limit = (Get-Date).AddDays(-90)
$files = Get-ChildItem -LiteralPath $dir -File -Recurse -ErrorAction SilentlyContinue |
         Where-Object { $_.LastWriteTime -lt $limit }

$bytes = 0; $count = 0
foreach ($f in $files) {
    if ($env:SIZESCOPE_DRYRUN -ne '1') {
        try { Remove-Item -LiteralPath $f.FullName -Force -ErrorAction Stop } catch { continue }
    }
    $bytes += $f.Length; $count++
}
# report to SizeScope: SIZESCOPE|<where>|<bytes>|<files>
Write-Output "SIZESCOPE|$dir|$bytes|$count"
