# pull the server value from the kubeconfig on host to construct our own kubeconfig, but using service principal settings
# this is required to build a kubeconfig using the kubeconfig on disk in c:\k, and the service principle granted in the container mount, to generate clientset
$cpEndpoint = Get-Content C:\k\config | ForEach-Object -Process { if ($_.Contains("server:")) { $_.Trim().Split()[1] } }
$token = Get-Content -Path $env:CONTAINER_SANDBOX_MOUNT_POINT\var\run\secrets\kubernetes.io\serviceaccount\token
$ca = Get-Content -Raw -Path $env:CONTAINER_SANDBOX_MOUNT_POINT\var\run\secrets\kubernetes.io\serviceaccount\ca.crt
$caBase64 = [System.Convert]::ToBase64String([System.Text.Encoding]::UTF8.GetBytes($ca))
$server = "server: $cpEndpoint"
(Get-Content $env:CONTAINER_SANDBOX_MOUNT_POINT\kubeconfigtemplate.yaml).
replace("<ca>", $caBase64).
replace("<server>", $server.Trim()).
replace("<token>", $token) | Set-Content $env:CONTAINER_SANDBOX_MOUNT_POINT\kubeconfig -Force

$env:KUBECONFIG = Join-Path -Path $env:CONTAINER_SANDBOX_MOUNT_POINT -ChildPath "kubeconfig"

# Set the KUBECONFIG environment variable 
[System.Environment]::SetEnvironmentVariable("KUBECONFIG", $env:KUBECONFIG, [System.EnvironmentVariableTarget]::Machine)
