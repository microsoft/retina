# retina-shell

Retina CLI provides a command to launch an interactive shell in a node or pod for adhoc debugging.

* The CLI command `kubectl retina shell` creates a pod with `HostNetwork=true` (for node debugging) or an ephemeral container in an existing pod (for pod debugging).
* The container runs an image built from the Dockerfile in this directory. The image is based on Azure Linux and includes commonly-used networking tools.

For testing, you can override the image used by `retina shell` either with CLI arguments
(`--retina-shell-image-repo` and `--retina-shell-image-version`) or environment variables
(`RETINA_SHELL_IMAGE_REPO` and `RETINA_SHELL_IMAGE_VERSION`).

Run `kubectl retina shell -h` for full documentation and examples.

Currently only Linux is supported; Windows support will be added in the future.
