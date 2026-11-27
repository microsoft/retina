$ErrorActionPreference = 'Stop'

# Load only the installer function; dot-sourcing the script would modify the host.
$tokens = $null
$parseErrors = $null
$ast = [System.Management.Automation.Language.Parser]::ParseFile(
    "$PSScriptRoot/install-ebpf-xdp.ps1", [ref] $tokens, [ref] $parseErrors)
if ($parseErrors.Count -gt 0) { throw ($parseErrors | Out-String) }
$function = $ast.Find({
    param($node)
    $node -is [System.Management.Automation.Language.FunctionDefinitionAst] -and $node.Name -eq 'Install-eBPF'
}, $true)
if ($null -eq $function) { throw 'Install-eBPF function not found' }
. ([ScriptBlock]::Create($function.Extent.Text))

$Script:eBPFVersion = '1.1.0'
$productCode = '{B6BCACB1-C872-4159-ABCB-43A50668056C}'

function Assert-TestSigningIsEnabled { return $true }
function Assert-SoftwareInstalled { param($ServiceName, [switch] $Silent) return $script:services -contains $ServiceName }
function Assert-eBPFVersion { param($Path) return $script:validDll }
function Invoke-WebRequest { param($Uri, $OutFile, $ErrorAction) }
function Get-ItemProperty { param($Path, $ErrorAction) return $script:registrations }
function Get-CimInstance {
    param($ClassName, $ErrorAction)
    foreach ($name in $script:services) { [PSCustomObject]@{ Name = $name; State = 'Stopped' } }
}
function Stop-Service { param($Name, [switch] $Force, $ErrorAction) }
function sc.exe {
    param($Action, $Name)
    if ($script:registrations.Count -ne 0) { throw 'Services deleted before MSI uninstall' }
    $script:operations.Add("delete:$Name")
    $script:services = @($script:services | Where-Object { $_ -ne $Name })
    $global:LASTEXITCODE = 0
}
function Start-Process {
    param($FilePath, $ArgumentList, [switch] $Wait, [switch] $PassThru, $ErrorAction)
    $script:operations.Add($ArgumentList[0])
    if ($ArgumentList[0] -eq '/x') {
        if ($ArgumentList[1] -ne $productCode) { throw 'Wrong registered product uninstalled' }
        if ($script:uninstallExit -eq 0) {
            $script:registrations = @()
            $script:services = @()
        }
        return [PSCustomObject]@{ ExitCode = $script:uninstallExit }
    }
    if ($script:registrations.Count -ne 0 -or $ArgumentList -contains 'REINSTALL=ALL') {
        throw 'Expected a fresh install, not repair or installation over a registered runtime'
    }
    if ($script:installExit -eq 0) {
        $script:services = @('eBPFCore', 'NetEbpfExt')
        $script:validDll = $true
    }
    return [PSCustomObject]@{ ExitCode = $script:installExit }
}
function Copy-Item {
    param($Path, $Destination, [switch] $Force, $ErrorAction)
    if (!$script:validDll) { throw 'Invalid runtime DLL copied' }
    $script:operations.Add('copy')
}

foreach ($case in @(
    @{ Name = 'clean node'; Services = @(); Expected = @('/i', 'copy'); Success = $true },
    @{ Name = 'healthy pinned runtime'; Version = '1.1.0'; ValidDll = $true; Services = @('eBPFCore', 'NetEbpfExt'); Expected = @('copy'); Success = $true },
    @{ Name = 'registered pinned runtime with missing DLL'; Version = '1.1.0'; Services = @('eBPFCore', 'NetEbpfExt'); Expected = @('/x', '/i', 'copy'); Success = $true },
    @{ Name = 'registered pinned runtime with missing service'; Version = '1.1.0'; ValidDll = $true; Services = @('eBPFCore'); Expected = @('/x', '/i', 'copy'); Success = $true },
    @{ Name = 'different registered version'; Version = '1.5.0'; Services = @('eBPFCore', 'NetEbpfExt'); Expected = @('/x', '/i', 'copy'); Success = $true },
    @{ Name = 'non-MSI node-image drivers'; Services = @('eBPFCore', 'NetEbpfExt'); Expected = @('delete:eBPFCore', 'delete:NetEbpfExt', '/i', 'copy'); Success = $true },
    @{ Name = 'uninstall failure preserves services'; Version = '1.5.0'; Services = @('eBPFCore', 'NetEbpfExt'); UninstallExit = 1603; Expected = @('/x'); Success = $false },
    @{ Name = 'uninstall requiring reboot stops installation'; Version = '1.1.0'; Services = @('eBPFCore'); UninstallExit = 3010; Expected = @('/x'); Success = $false },
    @{ Name = 'installation failure is returned'; Services = @(); InstallExit = 1603; Expected = @('/i'); Success = $false }
)) {
    $script:registrations = @()
    if ($case.Version) {
        $script:registrations = @([PSCustomObject]@{
            DisplayName = 'eBPF for Windows'; DisplayVersion = $case.Version; PSChildName = $productCode
        })
    }
    $script:services = $case.Services
    $script:validDll = [bool] $case.ValidDll
    $script:uninstallExit = [int] $case.UninstallExit
    $script:installExit = [int] $case.InstallExit
    $script:operations = [System.Collections.Generic.List[string]]::new()
    $result = @(Install-eBPF -LocalPath ([System.IO.Path]::GetTempPath()))
    if ($result.Count -ne 1 -or $result[0] -isnot [bool] -or $result[0] -ne $case.Success) {
        throw "$($case.Name): unexpected return value $result"
    }
    if (($script:operations -join ',') -ne ($case.Expected -join ',')) {
        throw "$($case.Name): unexpected operations $($script:operations -join ',')"
    }
    if ($case.UninstallExit -and ($script:services -join ',') -ne ($case.Services -join ',')) {
        throw "$($case.Name): services changed after failed uninstall"
    }
    Write-Host "PASS: $($case.Name)"
}
