#Requires -RunAsAdministrator

$ErrorActionPreference = 'Stop'
$readyPath = 'C:\install-ebpf-xdp-scale-probe-ready'

Remove-Item -Path $readyPath -Force -ErrorAction SilentlyContinue

& .\install-ebpf-xdp.ps1

& cmd.exe /c '.\event-writer-helper.bat EventWriter-Setup'
if ($LASTEXITCODE -ne 0) {
    throw "EventWriter-Setup failed with exit code $LASTEXITCODE"
}

& cmd.exe /c 'C:\event-writer-helper.bat EventWriter-LoadAndPinPrgAndMaps'
if ($LASTEXITCODE -ne 0) {
    throw "EventWriter-LoadAndPinPrgAndMaps failed with exit code $LASTEXITCODE"
}

New-Item -Path $readyPath -ItemType File -Force | Out-Null
