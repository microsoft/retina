FROM mcr.microsoft.com/windows/servercore:ltsc2022 As base

ARG EBPF_VERSION=1.1.0
ARG RETINA_EBPF_API_VERSION=1.6.0
ARG XDP_RUNTIME_VERSION=1.3.0
ENV EBPF_VERSION=${EBPF_VERSION} \
    RETINA_EBPF_API_VERSION=${RETINA_EBPF_API_VERSION} \
    XDP_RUNTIME_VERSION=${XDP_RUNTIME_VERSION}

SHELL ["powershell", "-NoLogo", "-Command"]

# Fetch install artifacts consumed by install-ebpf-xdp.ps1 from C:\install-artifacts.
# Each download is checked to be non-empty so a failed/blocked request fails the
# build instead of being baked silently into the image.
RUN $ErrorActionPreference = 'Stop'; $ProgressPreference = 'SilentlyContinue'; \
    function Get-ValidatedFile { \
        param([string] $Uri, [string] $OutFile) \
        Invoke-WebRequest -Uri $Uri -OutFile $OutFile -UseBasicParsing; \
        if ((Get-Item $OutFile).Length -eq 0) { throw "Downloaded file '$OutFile' is empty" } \
    }; \
    New-Item -ItemType Directory -Path C:\install-artifacts | Out-Null; \
    Get-ValidatedFile -Uri "https://github.com/microsoft/ebpf-for-windows/releases/download/Release-v$env:EBPF_VERSION/ebpf-for-windows.x64.$env:EBPF_VERSION.msi" -OutFile 'C:\install-artifacts\ebpf-for-windows.x64.msi'; \
    Get-ValidatedFile -Uri 'https://aka.ms/vs/17/release/vc_redist.x64.exe' -OutFile 'C:\install-artifacts\vc_redist.x64.exe'; \
    Get-ValidatedFile -Uri "https://www.nuget.org/api/v2/package/Microsoft.Wcn.Observability.eBPF.Retina.x64/$env:RETINA_EBPF_API_VERSION" -OutFile 'C:\install-artifacts\retinaebpfapi.zip'; \
    Get-ValidatedFile -Uri "https://www.nuget.org/api/v2/package/Microsoft.XDP-for-Windows.Runtime.x64/$env:XDP_RUNTIME_VERSION" -OutFile 'C:\install-artifacts\xdp-runtime.zip'

COPY ./install-ebpf-xdp.ps1 ./install-ebpf-xdp.ps1
COPY ./x64/Release/bpf_event_writer.sys ./bpf_event_writer.sys
COPY ./x64/Release/event_writer.exe ./event_writer.exe
COPY ./event-writer-helper.bat ./event-writer-helper.bat