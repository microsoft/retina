# Copyright (c) Microsoft Corporation.
# Licensed under the MIT license.
# example usage:
# powershell.exe -command "& { . .\windows.ps1; controller-image <imagetag> }"
# Retry({retina-image $(tag)-windows-amd64})

function Retry([Action]$action) {
    $attempts = 3    
    $sleepInSeconds = 5
    do {
        try {
            $action.Invoke();
            break;
        }
        catch [Exception] {
            Write-Host $_.Exception.Message
        }            
        $attempts--
        if ($attempts -gt 0) { 
            sleep $sleepInSeconds 
        }
    } while ($attempts -gt 0)    
}

function retina-image {
    if ($null -eq $env:TAG) { $env:TAG = $args[0] } 
    docker build `
        -f .\Dockerfile.windows `
        -t acnpublic.azurecr.io/retina-agent:$env:TAG `
        .
}
