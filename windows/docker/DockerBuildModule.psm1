# DockerBuildModule.psm1
$ErrorActionPreference = "Stop"
function Get-GitVersion {
    # Get the SHA of the current commit
    $sha = git rev-parse --short HEAD

    # Get the latest tag if available
    $tag = git describe --tags --abbrev=0 2>$null

    # Check if the current commit is tagged
    $isTaggedCommit = git tag --contains $sha

    if ($isTaggedCommit) {
        # If the current commit is tagged, return the tag
        return $tag
    }
    else {
        # If the current commit is not tagged, return the current tag followed by the SHA
        return $sha
    }
}


function Build-RetinaAgentImage {
    param(
        [string]$imageName = "retina-agent",
        [string]$tag = "$(Get-GitVersion)-windows-ltsc2022-amd64",
        [string]$appInsightsID = "",
        [Parameter(Mandatory = $true)][string]$fullBuilderImageName = "",
        [Parameter(Mandatory = $true)][string]$registry = ""
    )

    # Get the version using the Get-GitVersion function
    $version = Get-GitVersion

    # Hardcoded file path for Retina agent Dockerfile
    $filePath = "controller/Dockerfile.windows-native"

    # Full image name with registry, image name, and version
    $fullImageName = "${registry}/${imageName}:$tag"

    Write-Host "Building Retina agent Docker image $fullImageName with builder image $fullBuilderImageName"

    # Building the Retina agent Docker image with a build argument
    docker build -f $filePath -t $fullImageName --target final --build-arg BUILDER_IMAGE="$fullBuilderImageName" --build-arg VERSION=$version --build-arg APP_INSIGHTS_ID="$appInsightsID" .
}


function Save-Image {
    param(
        [Parameter(Mandatory = $true)][string]$imageName,
        [string]$tag = "$(Get-GitVersion)-windows-ltsc2022-amd64",
        [Parameter(Mandatory = $true)][string]$registry = "",
        [string]$directory = "./output/images/windows/amd64/2022"
    )


    New-Item -ItemType Directory -Path $directory -Force

    $savePath = "$directory/$imageName-$tag.tar"

    $fullImageName = "${registry}/${imageName}:$tag"

    Write-Host "Saving Docker image $fullImageName to $savePath"

    docker save -o $savePath $fullImageName
    Write-Host "Docker image saved to $savePath"
}

Export-ModuleMember -Function Get-GitVersion, Build-RetinaBuilderImage, Build-RetinaAgentImage, Save-Image
