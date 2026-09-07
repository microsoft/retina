@echo off
REM Add logic to call a specific function based on the argument
if "%1"=="EventWriter-Setup" goto EventWriter-Setup
if "%1"=="EventWriter-SetFilter" goto EventWriter-SetFilter
if "%1"=="EventWriter-GetRetinaPromMetrics" goto EventWriter-GetRetinaPromMetrics
if "%1"=="EventWriter-Curl" goto EventWriter-Curl
if "%1"=="EventWriter-LoadAndPinPrgAndMaps" goto EventWriter-LoadAndPinPrgAndMaps
if "%1"=="EventWriter-UnPinPrgAndMaps" goto EventWriter-UnPinPrgAndMaps
if "%1"=="EventWriter-Attach" goto EventWriter-Attach
if "%1"=="EventWriter-GetRetinaPromMetrics" goto EventWriter-GetPodIpAddress
if "%1"=="EventWriter-GetPodIpAddress" goto EventWriter-GetPodIpAddress
if "%1"=="EventWriter-GetPodIfIndex" goto EventWriter-GetPodIfIndex
goto :EOF

:EventWriter-Setup
   copy .\event_writer.exe C:\event_writer.exe
   copy .\bpf_event_writer.sys C:\bpf_event_writer.sys
   goto :EOF

:EventWriter-SetFilter
   set PREV_DIR=%CD%
   cd C:\
   .\event_writer.exe -set-filter -event %3 -srcIP %5 -ifindx %7
   cd /d %PREV_DIR%
   goto :EOF

:EventWriter-Attach
   set PREV_DIR=%CD%
   cd C:\
   .\event_writer.exe -attach -ifindx %2
   cd /d %PREV_DIR%
   goto :EOF

:EventWriter-LoadAndPinPrgAndMaps
   set PREV_DIR=%CD%
   cd C:\
   .\event_writer.exe -load-pin
   cd /d %PREV_DIR%
   goto :EOF

:EventWriter-UnPinPrgAndMaps
   set PREV_DIR=%CD%
   cd C:\
   .\event_writer.exe -unpin
   cd /d %PREV_DIR%
   goto :EOF

:EventWriter-GetRetinaPromMetrics
   curl -s http://localhost:10093/metrics
   goto :EOF

:EventWriter-Curl
   curl http://%2
   goto :EOF

:EventWriter-GetPodIpAddress
   powershell -command "Get-NetIPAddress | Where-Object {$_.AddressFamily -eq 'IPv4' -and $_.IPAddress -ne '127.0.0.1'} | Select-Object -ExpandProperty IPAddress"
   goto :EOF

:EventWriter-GetPodIfIndex
   powershell -command "Get-NetAdapter | Where-Object { $_.InterfaceDescription -like '*Hyper-V Virtual Ethernet Container*' } | ForEach-Object { Write-Output $_.ifIndex }"
   goto :EOF