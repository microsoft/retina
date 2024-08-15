# DevContainer Setup

This folder contains the configuration files for setting up the development container. The devcontainer will start with a kind cluster and a kind registry. You can build and push images to this kind registry and install onto your kind cluster.

Example:

```bash
# Build and create retina-agent, retina-init, retina-operator images with test tag
make quick-build-devcontainer RETINA_PLATFORM_TAG=test
# Pushes images to the kind registry and deletes the local images with test tag
make quick-push-devcontainer RETINA_PLATFORM_TAG=test
# deploys retina on hubble dataplane to devcontainer with test tag
make quick-deploy-hubble-devcontainer RETINA_PLATFORM_TAG=test
```

## Files

- `devcontainer.json`: Main configuration file for the DevContainer.
- `installMoreTools.sh`: Script to install additional tools in the DevContainer.
- `setupKindWithLocalRegistry.sh`: Script to set up a local Kubernetes cluster with Kind and a local Docker registry.
- `daemon.json`: Docker daemon configuration file.

## Setup Instructions

1. Ensure you have Docker and Visual Studio Code installed.
2. Open the project in Visual Studio Code.
3. Rebuild and reopen the DevContainer:
   - Open the Command Palette (`Ctrl+Shift+P`).
   - Select `Remote-Containers: Rebuild and Reopen in Container`.
