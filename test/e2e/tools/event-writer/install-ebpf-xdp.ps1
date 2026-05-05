#Requires -RunAsAdministrator

# This script performs Windows node setup required for Retina e2e tests.

# Version configuration
$Script:eBPFVersion = "1.1.0"
$Script:RetinaEbpfAPIVersion = "1.3.0"
$Script:XDPRuntimeVersion = "1.3.0"

Function Assert-SoftwareInstalled
{
   [cmdletbinding(DefaultParameterSetName='Software')]

   Param
   (
      [Parameter(ParameterSetName='Service',Mandatory=$true)]
      [ValidateScript({-Not [String]::IsNullOrWhiteSpace($_)})]
      [String] $ServiceName,

      [Parameter(ParameterSetName='Service',Mandatory=$false)]
      [ValidateSet($null,'Running','Stopped')]
      [String] $ServiceState,

      [Parameter(ParameterSetName='Software',Mandatory=$true)]
      [ValidateScript({-Not [String]::IsNullOrWhiteSpace($_)})]
      [String] $SoftwareName,

      [Parameter(ParameterSetName='Software',Mandatory=$false)]
      [String] $SoftwareVersion,

      [Parameter(ParameterSetName='Service',Mandatory=$false)]
      [Parameter(ParameterSetName='Software',Mandatory=$false)]
      [Switch] $Silent
   )

   [String] $name = If($ServiceName) {"$($ServiceName)"}Else{"$($SoftwareName)"}

   If(-Not $Silent.IsPresent)
   {
       Write-Host -Object:"Checking if $($name) is installed ..."
   }

   [Boolean] $isInstalled = $false

   Try
   {
      If($SoftwareName)
      {
         $software = Get-WmiObject -Class:'Win32_Product' | Where-Object -Property:'Name' -like "*$($SoftwareName)*"

         If($software -And
            (-Not [String]::IsNullOrWhiteSpace($SoftwareVersion)))
         {
            $software = $software | Where-Object -Property:'Version' -like "*$($SoftwareVersion)*"
         }

         If($software)
         {
           $isInstalled = $true
         }
      }
      ElseIF($ServiceName)
      {
        [Object] $state = Get-Service -Name:"$($ServiceName)" -ErrorAction:'SilentlyContinue'
        If($state)
        {
           $isInstalled = $true

           If($ServiceState -And
              -Not ($state.Status -INE $ServiceState))
           {
              Write-Warning -Message:"`t$ServiceName is $$(state.Status)"
           }
        }
      }
   }
   Catch
   {

   }

   If(-Not $Silent.IsPresent)
   {
      If($isInstalled)
      {
         Write-Host -Object:"`t$($name) is installed"
      }
      Else
      {
         Write-Host -Object:"`t$($name) is not installed"
      }
   }

   Return $isInstalled
}

<#
 .Name
   Assert-TestSigningIsEnabled

 .Synopsis
   Internal cmdlet to check if testsigning is enabled in the boot loader.

 .Description
   Returns TRUE if test signing is enabled, otherwise FALSE.

 .Parameter Silent
   Optional switch used to suppress output messages

 .Example
   # Check if test signing is enabled
   Assert-TestSigningIsEnabled
#>
Function Assert-TestSigningIsEnabled
{
   Param
   (
      [Parameter(ParameterSetName='Default',Mandatory=$false)]
      [Switch] $Silent
   )

   [Boolean] $isEnabled = $false
   [String]  $state     = 'Disabled'

   Try
   {
      [Boolean] $current = $false

      If(-Not ($Silent.IsPresent))
      {
         Write-Host -Object:"`tAssert Test Signing is Enabled"
      }

      [Object[]] $entries = BCDEdit.exe /enum
      If($entries.Count -ILT 3)
      {
         Write-Error -Message:"$entries"

         Throw
      }

      ForEach($entry in $entries)
      {
         If($entry.StartsWith('identifier'))
         {
            If($entry -ILike '*{current}*')
            {
               $current = $true
            }
            Else
            {
               $current = $false
            }
         }
         Else
         {
            If($current)
            {
               If($entry -ILike '*testsigning*Yes*')
               {
                  $state = 'Enabled'

                  $isEnabled = $true

                  Break
               }
            }
         }
      }
   }
   Catch
   {
      $isEnabled = $false

      $state = 'Unknown'
   }

   If(-Not ($Silent.IsPresent))
   {
      Write-Host -Object:"`t`t$($state)"
   }

   Return $isEnabled
}

<#
 .Name
   Disable-TestSigning

 .Synopsis
   Internal cmdlet to turn off Test Signing in the Windows Boot Loader.

 .Description
   Returns TRUE if test signing is disabled, otherwise FALSE.
   If set, the setting does not take effect until a reboot

 .Parameter Reboot
   Optional parameter which will trigger a reboot if needed

 .Example
   # Disable test signing
   Disable-TestSigning
#>
Function Disable-TestSigning
{
   Param
   (
      [Parameter(ParameterSetName='Default',Mandatory=$false)]
      [Switch] $Reboot
   )

   [Boolean] $isSuccess = $true

   Try
   {
      [Boolean] $current = $false
      [Boolean] $found   = $false

      Write-Host -Object:"`tDisabling Test Signing"

      If(Assert-TestSigningIsEnabled -Silent)
      {
         Start-Process -FilePath:"$($env:WinDir)\System32\BCDEdit.exe" -ArgumentList @('/Set TestSigning Off') -PassThru | Wait-Process

         If(Assert-TestSigningIsEnabled -Silent)
         {
            Write-Error -Message:"`t`tFailed"

            Throw
         }

         $script:RequiresReboot = $true
      }

      Write-Host -Object:"`t`tDisabled"
   }
   Catch
   {
      $isSuccess = $false
   }

   If($Reboot.IsPresent -and
      $script:RequiresReboot)
   {
      Write-Host -Object:'Restarting'

      Start-Sleep -Seconds:5

      Restart-Computer
      Start-Sleep -Seconds:60
   }

   Return $isSuccess
}

<#
 .Name
   Enable-TestSigning

 .Synopsis
   Internal cmdlet to turn on Test Signing in the Windows Boot Loader.

 .Description
   Returns TRUE if test signing is enabled, otherwise FALSE.

 .Parameter Reboot
   Optional parameter which will trigger a reboot if needed

 .Example
   # Enable test signing
   Enable-TestSigning
#>
Function Enable-TestSigning
{
   Param
   (
      [Parameter(ParameterSetName='Default',Mandatory=$false)]
      [Switch] $Reboot
   )

   [Boolean] $isSuccess = $true

   Try
   {
      [Boolean] $current = $false
      [Boolean] $found   = $false

      Write-Host -Object:"`tEnabling Test Signing"

      If(-Not (Assert-TestSigningIsEnabled -Silent))
      {
         Start-Process -FilePath:"$($env:WinDir)\System32\BCDEdit.exe" -ArgumentList @('/Set TestSigning On') -PassThru | Wait-Process

         If(-Not (Assert-TestSigningIsEnabled -Silent))
         {
            Write-Error -Message:"`t`tFailed"

            Throw
         }

         $script:RequiresReboot = $true
      }

      Write-Host -Object:"`t`tEnabled"

   }
   Catch
   {
      Write-Host "Enable-TestSigning : $_"
      $isSuccess = $false
   }

   If($Reboot.IsPresent -and
      $script:RequiresReboot)
   {
      Write-Host -Object:'Restarting'

      Start-Sleep -Seconds:5

      Restart-Computer
      Start-Sleep -Seconds:60
   }

   Return $isSuccess
}

#endregion PrivateFns

#region Public

<#
 .Name
   Assert-WindowsEbpfXdpIsReady

 .Synopsis
   Check if EBPF and XDP for Windows is ready

 .Description
   Returns TRUE if EBPF and XDP for Windows is ready, otherwise FALSE.

 .Example
   # Check if EBPF and XDP for Windows is ready
   Assert-WindowsCiliumFunctions
#>
Function Assert-WindowsEbpfXdpIsReady
{
    Write-Host -Object:'Validating EBPF and XDP for Windows is ready'

   [Boolean]  $isReady  = $true
   [String[]] $services = @(
                            'eBPFCore',
                            'NetEbpfExt',
                            'XDP'
                           )
   ForEach($service in $services)
   {
      If(-Not (Assert-SoftwareInstalled -ServiceName:"$($service)" -ServiceState:'Running'))
      {
         $isReady = $false

         Write-Warning -Message:"`t$($service) is not ready"
      }
   }

   # Verify VC++ Runtime DLLs
   $requiredDlls = @("MSVCP140.dll", "VCRUNTIME140.dll", "VCRUNTIME140_1.dll")
   ForEach($dll in $requiredDlls)
   {
      If(-Not (Test-Path "$env:WinDir\System32\$dll"))
      {
         $isReady = $false
         Write-Warning -Message:"`t$dll is not present in System32"
      }
   }

   # Verify EbpfApi.dll in System32
   If(-Not (Test-Path "$env:WinDir\System32\EbpfApi.dll"))
   {
      $isReady = $false
      Write-Warning -Message:"`tEbpfApi.dll is not present in System32"
   }

   # Verify retinaebpfapi.dll in System32
   If(-Not (Test-Path "$env:WinDir\System32\retinaebpfapi.dll"))
   {
      $isReady = $false
      Write-Warning -Message:"`tretinaebpfapi.dll is not present in System32"
   }

   Return $isReady
}

<#
 .Name
   Install-eBPF

 .Synopsis
   Installs extended Berkley Packet Filter for Windows.

 .Description
   Returns TRUE if extended Berkley Packet Filter for Windows is installed successfully, otherwise FALSE.
   Function requires that Test Signing is enabled.

 .Parameter LocalPath
   Local directory to the eBPF for Windows binaries.
   Default location is $env:LocalAppData\Temp

 .Example
   # Install eBPF for Windows
   Install-eBPF -LocalPath:"$env:TEMP"
#>
Function Install-eBPF
{
   [cmdletbinding(DefaultParameterSetName='Default')]

   Param
   (
      [Parameter(ParameterSetName='Default',Mandatory=$false)]
      [ValidateScript({Test-Path $_ -PathType:'Container'})]
      [String] $LocalPath = "$env:TEMP"
   )

   [Boolean] $isSuccess = $true

   Try
   {
      Write-Host 'Installing extended Berkley Packet Filter for Windows'
      If(-Not (Assert-TestSigningIsEnabled))
      {
         If(-Not (Enable-TestSigning -Reboot)) { Throw }
      }

      If(Assert-SoftwareInstalled -ServiceName:"eBPFCore")
      {
         Write-Host 'extended Berkley Packet Filter for Windows is already installed'
         return $isSuccess
      }

      Write-Host 'Installing extended Berkley Packet Filter for Windows'
      # Download eBPF-for-Windows.
      $packageEbpfUrl  = "https://github.com/microsoft/ebpf-for-windows/releases/download/Release-v$Script:eBPFVersion/ebpf-for-windows.x64.$Script:eBPFVersion.msi"
      Invoke-WebRequest -Uri $packageEbpfUrl -OutFile "$LocalPath\ebpf-for-windows.x64.$Script:eBPFVersion.msi"

      Start-Process -FilePath "$($env:WinDir)\System32\MSIExec.exe" -ArgumentList @("/i", "$LocalPath\ebpf-for-windows.x64.$Script:eBPFVersion.msi", "/qn", "INSTALLFOLDER=`"$($env:ProgramFiles)\ebpf-for-windows`"", "ADDLOCAL=eBPF_Runtime_Components") -PassThru | Wait-Process
      If(-Not (Assert-SoftwareInstalled -ServiceName:'eBPFCore' -Silent) -Or
         -Not (Assert-SoftwareInstalled -ServiceName:'NetEbpfExt' -Silent))
      {
         Write-Error -Message:"`eBPF service failed to install"
         Throw
      }

      $isSuccess = Assert-SoftwareInstalled -ServiceName:"eBPFCore"

      # TODO : Remove this once retinaebpfapi.dll can find EbpfApi.dll from the install location.
      # Copy EbpfApi.dll to System32 so dependent DLLs can find it
      $ebpfApiSource = "$($env:ProgramFiles)\ebpf-for-windows\EbpfApi.dll"
      $ebpfApiDest = "$env:WinDir\System32\EbpfApi.dll"
      If((Test-Path $ebpfApiSource) -And -Not (Test-Path $ebpfApiDest))
      {
         Copy-Item -Path $ebpfApiSource -Destination $ebpfApiDest -Force
         Write-Host "EbpfApi.dll copied to $ebpfApiDest"
      }
   }
   Catch
   {
      $isSuccess = $false
      Write-Host "EBPF install failed : $_"
      Uninstall-eBPF
   }

   Return $isSuccess
}

<#
 .Name
   Install-VCRuntime

 .Synopsis
   Installs the Visual C++ Runtime redistributable.

 .Description
   Downloads and installs the Microsoft Visual C++ Redistributable (x64) which provides
   MSVCP140.dll, VCRUNTIME140.dll, and VCRUNTIME140_1.dll in C:\Windows\System32. retinaebpfapi.dll
   depends on these DLLs. Returns TRUE if successful, otherwise FALSE.
#>
Function Install-VCRuntime
{
   [Boolean] $isSuccess = $true

   Try
   {
      $requiredDlls = @("MSVCP140.dll", "VCRUNTIME140.dll", "VCRUNTIME140_1.dll")
      $allPresent = $true

      ForEach($dll in $requiredDlls)
      {
         If(-Not (Test-Path "$env:WinDir\System32\$dll"))
         {
            $allPresent = $false
            Break
         }
      }

      If($allPresent)
      {
         Write-Host 'Visual C++ Runtime DLLs are already installed'
         return $isSuccess
      }

      Write-Host 'Installing Visual C++ Redistributable (x64)'

      $vcRedistUrl = "https://aka.ms/vs/17/release/vc_redist.x64.exe"
      $vcRedistPath = "$env:TEMP\vc_redist.x64.exe"

      Invoke-WebRequest -Uri $vcRedistUrl -OutFile $vcRedistPath
      Start-Process -FilePath $vcRedistPath -ArgumentList @("/install", "/quiet", "/norestart") -Wait

      # Verify installation
      ForEach($dll in $requiredDlls)
      {
         If(-Not (Test-Path "$env:WinDir\System32\$dll"))
         {
            Write-Error "$dll not found after VC++ Redistributable install"
            Throw
         }
      }

      Write-Host 'Visual C++ Runtime DLLs installed successfully'
   }
   Catch
   {
      $isSuccess = $false
      Write-Host "Visual C++ Runtime install failed: $_"
   }
   Finally
   {
      Remove-Item -Path "$env:TEMP\vc_redist.x64.exe" -Force -ErrorAction SilentlyContinue
   }

   Return $isSuccess
}

<#
 .Name
   Install-RetinaEbpfAPI

 .Synopsis
   Downloads and installs retinaebpfapi.dll from the NuGet gallery.

 .Description
   Downloads the Microsoft.Wcn.Observability.eBPF.Retina.x64 NuGet package and
   copies retinaebpfapi.dll to C:\Windows\System32.
   Returns TRUE if successful, otherwise FALSE.
#>
Function Install-RetinaEbpfAPI
{
   [Boolean] $isSuccess = $true

   Try
   {
      $dllDest = "$env:WinDir\System32\retinaebpfapi.dll"

      If(Test-Path $dllDest)
      {
         Write-Host 'retinaebpfapi.dll is already installed'
         return $isSuccess
      }

      Write-Host 'Installing retinaebpfapi.dll from NuGet'

      $nugetUrl = "https://www.nuget.org/api/v2/package/Microsoft.Wcn.Observability.eBPF.Retina.x64/$Script:RetinaEbpfAPIVersion"
      $zipPath = "$env:TEMP\eBPFRetina.zip"
      $extractPath = "$env:TEMP\eBPFRetina"

      Invoke-WebRequest -Uri $nugetUrl -OutFile $zipPath
      Expand-Archive -Path $zipPath -DestinationPath $extractPath -Force

      $dllSource = "$extractPath\build\native\bin\retinaebpfapi.dll"
      If(-Not (Test-Path $dllSource))
      {
         Write-Error "retinaebpfapi.dll not found in NuGet package at $dllSource"
         Throw
      }

      Copy-Item -Path $dllSource -Destination $dllDest -Force
      Write-Host "retinaebpfapi.dll installed to $dllDest"
   }
   Catch
   {
      $isSuccess = $false
      Write-Host "retinaebpfapi.dll install failed: $_"
   }
   Finally
   {
      # Cleanup
      Remove-Item -Path "$env:TEMP\eBPFRetina.zip" -Force -ErrorAction SilentlyContinue
      Remove-Item -Path "$env:TEMP\eBPFRetina" -Recurse -Force -ErrorAction SilentlyContinue
   }

   Return $isSuccess
}

<#
 .Name
   Install-XDP

 .Synopsis
   Installs the eXpress Data Path for Windows service.

 .Description
   Returns TRUE if the eXpress Data Path for Windows service is installed successfully, otherwise FALSE.

 .Parameter LocalPath
   Local directory to the eXpress Data Path for Windows service binaries.
   Default location is $env:LocalAppData\Temp

 .Example
   # Install the eXpress Data Path service
   Install-XDP -LocalPath:"$env:TEMP"
#>
Function Install-XDP
{
   [cmdletbinding(DefaultParameterSetName='Default')]

   Param
   (
      [Parameter(ParameterSetName='Default',Mandatory=$false)]
      [ValidateScript({Test-Path $_ -PathType:'Container'})]
      [String] $LocalPath = "$env:TEMP"
   )

   [Boolean] $isSuccess = $true

   Try
   {
      If(Assert-SoftwareInstalled -ServiceName:'XDP' -Silent)
      {
         Write-Host 'XDP for Windows is already installed'
         return $isSuccess
      }

      # Download and extract the XDP runtime NuGet package.
      Write-Host 'Installing eXpress Data Path for Windows'
      $xdpRuntimeVersion = $Script:XDPRuntimeVersion
      $xdpNupkgUrl = "https://www.nuget.org/api/v2/package/Microsoft.XDP-for-Windows.Runtime.x64/$xdpRuntimeVersion"
      $xdpZipPath = "$LocalPath\Microsoft.XDP-for-Windows.Runtime.x64.$xdpRuntimeVersion.zip"
      $xdpExtractPath = "$LocalPath\xdp-runtime"

      Invoke-WebRequest -Uri $xdpNupkgUrl -OutFile $xdpZipPath
      Expand-Archive -Path $xdpZipPath -DestinationPath $xdpExtractPath -Force
      Remove-Item -Path $xdpZipPath -Force

      # Install XDP using xdp-setup.ps1 from the runtime package
      $xdpSetupScript = Get-ChildItem -Path $xdpExtractPath -Recurse -Filter "xdp-setup.ps1" | Select-Object -First 1
      If($null -eq $xdpSetupScript) {
         Write-Error -Message:"xdp-setup.ps1 not found in the runtime package"
         Throw
      }

      # Trust the certificate from xdp.sys so Windows allows the driver to load
      $xdpSys = Get-ChildItem -Path $xdpExtractPath -Recurse -Filter "xdp.sys" | Select-Object -First 1
      If($null -ne $xdpSys) {
         $xdpCert = (Get-AuthenticodeSignature $xdpSys.FullName).SignerCertificate
         If($null -ne $xdpCert) {
            $xdpCertPath = "$LocalPath\xdp.cer"
            Export-Certificate -Cert $xdpCert -FilePath $xdpCertPath -Type CERT -Force
            certutil -f -addstore Root $xdpCertPath
            certutil -f -addstore TrustedPublisher $xdpCertPath
            Remove-Item -Path $xdpCertPath -Force
         } Else {
            Write-Warning "xdp.sys is not signed, skipping certificate trust"
         }
      } Else {
         Write-Warning "xdp.sys not found in the runtime package, skipping certificate trust"
      }

      & $xdpSetupScript.FullName -Install xdp
      & $xdpSetupScript.FullName -Install xdpebpf

      reg.exe add "HKLM\SYSTEM\CurrentControlSet\Services\xdp\Parameters" /v XdpEbpfEnabled /d 1 /t REG_DWORD /f
      net.exe stop xdp
      net.exe start xdp

      If(-Not (Assert-SoftwareInstalled -ServiceName:'XDP' -Silent)) {
         Throw
      }

   }
   Catch
   {
      $isSuccess = $false
      Write-Host "XDP install failed : $_"
      Uninstall-XDP
   }

   Return $isSuccess
}

<#
 .Name
   Install-EbpfXdp

 .Synopsis
   Installs EBPF and XDP for Windows

 .Description
   Returns TRUE if EBPF and XDP for Windows is installed successfully, otherwise FALSE.

 .Example
   # Install EBPF and XDP for Windows
   Install-EbpfXdp
#>
Function Install-EbpfXdp
{
   Try
   {
      If(Assert-WindowsEbpfXdpIsReady) {
         Write-Host 'eBPF and XDP for Windows is installed successfully'
         write-Host 'Create the probe ready file'
         # Create the probe ready file
         New-Item -Path "C:\install-ebpf-xdp-probe-ready" -ItemType File -Force 
         return
      }

      If(-Not (Assert-TestSigningIsEnabled -Silent))
      {
         If(-Not (Enable-TestSigning -Reboot)) {Throw}
      }
      
      $hnsPath = "HKLM:\SYSTEM\CurrentControlSet\Services\hns\State"
      $valueName = "CiliumOnWindows"

      if (-not (Test-Path $hnsPath)) {
         New-Item -Path $hnsPath -Force | Out-Null
      }

      $existing = Get-ItemProperty -Path $hnsPath -Name $valueName -ErrorAction SilentlyContinue
   
      If ($null -eq $existing) {
         Write-Host "CiliumOnWindows not found, creating it"
         New-ItemProperty -Path $hnsPath -Name $valueName -PropertyType DWORD -Value 1 -Force 
      } else {
         If ($existing.CiliumOnWindows -ne 1) {
            Write-Host "Setting CiliumOnWindows to 1"
            Set-ItemProperty -Path $hnsPath -Name $valueName -PropertyType DWORD -Value 1 -Force 
         }
      }
     
      If(-Not (Install-eBPF)) {Throw}

      If(-Not (Install-XDP)) {Throw}

      If(-Not (Install-VCRuntime)) {Throw}

      If(-Not (Install-RetinaEbpfAPI)) {Throw}

      Write-Host 'eBPF and XDP for Windows is installed successfully'
      write-Host 'Create the probe ready file'
      # Create the probe ready file
      New-Item -Path "C:\install-ebpf-xdp-probe-ready" -ItemType File -Force
   }
   Catch
   {
      $isSuccess = $false
   }

   return $isSuccess
}

<#
 .Name
   Uninstall-eBPF

 .Synopsis
   Uninstalls the extended Berkley Packet Filter for Windows.

 .Description
   Returns TRUE if the extended Berkley Packet Filter for Windows is uninstalled successfully, otherwise FALSE.

 .Parameter LocalPath
   Local directory to the extended Berkley Packet Filter for Windows binaries.
   Default location is $env:LocalAppData\Temp

 .Example
   # Uninstall the extended Berkley Packet Filter for Windows
   Uninstall-eBPF -LocalPath:"$(env:LocalAppData)\Temp"
#>
Function Uninstall-eBPF
{
   [cmdletbinding(DefaultParameterSetName='Default')]

   Param
   (
      [Parameter(ParameterSetName='Default',Mandatory=$false)]
      [ValidateScript({Test-Path $_ -PathType:'Container'})]
      [String] $LocalPath = "$env:TEMP"
   )

   Write-Host 'Uninstalling the extended Berkley Packet Filter for Windows'

   [Boolean] $isSuccess = $true

   Try
   {
      [String[]] $services = @('eBPFCore',
                               'NetEbpfExt'
                              )

      ForEach($service in $services)
      {
         [Object] $state = Get-Service -Name:$($service) -ErrorAction:'SilentlyContinue'
         If($state)
         {
            For([Byte]$i = 0;
                $i -ILE 5;
                $i++)
            {
               If($state.Status -IEQ 'Stopped')
               {
                   Break
               }
               Else
               {
                  If($state.Status -IEQ 'Running')
                  {
                     Stop-Service -Name:"$($service)" -Force
                  }
                  ElseIf($state.Status -IEQ 'StopPending')
                  {
                     Start-Sleep -Seconds:5
                  }
                  Else
                  {
                     Write-Error -Message:"$($service) service is $($state.status)"

                     Throw
                  }
               }

               $state = Get-Service -Name:"$($service)"
            }
         }

         Start-Process -FilePath:"$($env:WinDir)\System32\MSIExec.exe" -ArgumentList @("/x $($LocalPath)\ebpf-for-windows.x64.$Script:eBPFVersion.msi", '/qn') -PassThru | Wait-Process
      }

      If((Assert-SoftwareInstalled -ServiceName:'eBPFCore' -Silent) -or
         (Assert-SoftwareInstalled -ServiceName:'NetEbpfExt' -Silent) -or
         (Assert-SoftwareInstalled -SoftwareName:'eBPF for Windows' -Silent))
      {
         Write-Error -Message:"eBPF for Windows is still installed"

         Throw
      }
   }
   Catch
   {
      $isSuccess = $false
   }

   Return $isSuccess
}

<#
 .Name
   Uninstall-XDP

 .Synopsis
   Uninstalls the express Data Path for Windows service

 .Description
   Returns TRUE if the eXpress Data Path for Windows service is uninstalled successfully, otherwise FALSE.

 .Parameter LocalPath
   Local directory to the eXpress Data Path for Windows service binaries.
   Default location is $env:LocalAppData\Temp

 .Example
   # Uninstall the eXpress Data Path for Windows service
   Uninstall-XDP -LocalPath:"$($env:LocalAppData)\Temp"
#>
Function Uninstall-XDP
{
   [cmdletbinding(DefaultParameterSetName='Default')]

   Param
   (
      [Parameter(ParameterSetName='Default',Mandatory=$false)]
      [ValidateScript({Test-Path $_ -PathType:'Container'})]
      [String] $LocalPath = "$env:TEMP"
   )

   Write-Host 'Uninstalling eXpress Data Path for Windows'

   [Boolean] $isSuccess = $true

   Try
   {
      [Object] $state = Get-Service -Name:'XDP' -ErrorAction:'SilentlyContinue'
      If($state)
      {
         For([Byte]$i = 0;
             $i -ILE 5;
             $i++)
         {
            If($state.Status -IEQ 'Stopped')
            {
                Break
            }
            Else
            {
               If($state.Status -IEQ 'Running')
               {
                  Stop-Service -Name:'XDP' -Force
               }
               ElseIf($state.Status -IEQ 'StopPending')
               {
                  Start-Sleep -Seconds:5
               }
               Else
               {
                  Write-Error -Message:"XDP service is $($state.status)"

                  Throw
               }
            }

            $state = Get-Service -Name:'XDP'
         }

         # Uninstall using xdp-setup.ps1 from the extracted runtime package
         $xdpExtractPath = "$LocalPath\xdp-runtime"
         $xdpSetupScript = Get-ChildItem -Path $xdpExtractPath -Recurse -Filter "xdp-setup.ps1" -ErrorAction SilentlyContinue | Select-Object -First 1
         If($null -ne $xdpSetupScript) {
            & $xdpSetupScript.FullName -Uninstall xdpebpf
            & $xdpSetupScript.FullName -Uninstall xdp
         }
      }

      If((Assert-SoftwareInstalled -ServiceName:'XDP' -Silent))
      {
         Write-Error -Message:"XDP for Windows is still installed"

         Throw
      }
   }
   Catch
   {
      $isSuccess = $false
   }

   Return $isSuccess
}


#Script Start
exit $(Install-EbpfXdp)