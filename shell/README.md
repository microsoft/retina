# retina-shell

Retina CLI provides a command to launch an interactive shell in a node or pod for adhoc debugging.

* The CLI command `kubectl retina shell` creates a pod with `HostNetwork=true` (for node debugging) or an ephemeral container in an existing pod (for pod debugging).
* For Linux nodes, the container runs an image built from the Dockerfile in this directory, based on Azure Linux and includes commonly-used networking tools.
* For Windows nodes, the container runs a Windows-based image with Windows networking utilities built from Dockerfile.windows.

For testing, you can override the image used by `retina shell` either with CLI arguments
(`--retina-shell-image-repo` and `--retina-shell-image-version`) or environment variables
(`RETINA_SHELL_IMAGE_REPO` and `RETINA_SHELL_IMAGE_VERSION`).

For Windows nodes, you can specify the Windows image tag suffix with the `--windows-image-tag` flag or
the `RETINA_SHELL_WINDOWS_IMAGE_TAG` environment variable.

Run `kubectl retina shell -h` for full documentation and examples.
