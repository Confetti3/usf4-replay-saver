param([Parameter(Mandatory = $true)][string]$Version)
$ErrorActionPreference = 'Stop'
Set-Location $PSScriptRoot

$dist = Join-Path $PSScriptRoot 'dist'
if (Test-Path $dist) { Remove-Item -Recurse -Force $dist }
New-Item -ItemType Directory $dist | Out-Null

$targets = @(
    @{ os = 'windows'; arch = 'amd64'; exe = 'usf4-replay-saver.exe' },
    @{ os = 'linux'; arch = 'amd64'; exe = 'usf4-replay-saver' }
)
$env:CGO_ENABLED = '0'
foreach ($t in $targets) {
    $name = "usf4-replay-saver-$($t.os)-$($t.arch)"
    $stage = Join-Path $dist $name
    New-Item -ItemType Directory $stage | Out-Null
    $env:GOOS = $t.os
    $env:GOARCH = $t.arch
    $ldflags = "-s -w -X main.version=$Version"
    if ($t.os -eq 'windows') { $ldflags += ' -H windowsgui' }
    go build -trimpath -ldflags $ldflags -o (Join-Path $stage $t.exe) .
    if ($LASTEXITCODE -ne 0) { throw "build failed for $name" }
    Copy-Item README.md, LICENSE $stage
    if ($t.os -eq 'windows') {
        Compress-Archive -Path (Join-Path $stage '*') -DestinationPath "$stage.zip"
        $archive = "$stage.zip"
    } else {
        # An mtree spec lets Windows tar mark the binary executable.
        $spec = Join-Path $dist 'linux.mtree'
        @(
            '#mtree',
            "$name type=dir mode=0755",
            "$name/$($t.exe) type=file mode=0755 contents=$name/$($t.exe)",
            "$name/README.md type=file mode=0644 contents=$name/README.md",
            "$name/LICENSE type=file mode=0644 contents=$name/LICENSE"
        ) | Set-Content -Encoding ascii $spec
        Push-Location $dist
        & "$env:SystemRoot\System32\tar.exe" -czf "$name.tar.gz" "@linux.mtree"
        $tarExit = $LASTEXITCODE
        Pop-Location
        Remove-Item $spec
        if ($tarExit -ne 0) { throw "tar failed for $name" }
        $archive = "$stage.tar.gz"
    }
    $hash = (Get-FileHash -Algorithm SHA256 $archive).Hash.ToLower()
    "$hash  $(Split-Path -Leaf $archive)" | Set-Content -NoNewline -Encoding ascii "$archive.sha256"
}
Remove-Item Env:GOOS, Env:GOARCH, Env:CGO_ENABLED
Get-ChildItem $dist -File | Select-Object Name, Length
