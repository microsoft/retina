# Import the DockerBuildModule
#Import-Module -Name .\DockerBuildModule.psm1
$ErrorActionPreference = "Stop"

Build-RetinaBuilderImage -version "v2"

# Get the version for the Retina builder image
$builderVersion = Get-GitVersion

# Define the builder image version you want to use
#$customBuilderImageVersion = "custom-version"

# Define the full custom builder image name with the custom version
$customBuilderImageName = "$defaultRegistry/retina-builder:$customBuilderImageVersion"

# Call the Build-RetinaAgentImage function with the custom builder image name
Build-RetinaAgentImage
