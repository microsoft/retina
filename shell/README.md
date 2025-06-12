# retina-shell

Retina CLI provides a command to launch an interactive shell in a node or pod for adhoc debugging.

* The CLI command `kubectl retina shell` creates a pod with `HostNetwork=true` (for node debugging) or an ephemeral container in an existing pod (for pod debugging).
* The container runs an image built from the Dockerfile in this directory. The image is based on Azure Linux and includes commonly-used networking tools.
* The [pwru](https://github.com/cilium/pwru) tool is bundled in the image for advanced kernel packet tracing.

For testing, you can override the image used by `retina shell` either with CLI arguments
(`--retina-shell-image-repo` and `--retina-shell-image-version`) or environment variables
(`RETINA_SHELL_IMAGE_REPO` and `RETINA_SHELL_IMAGE_VERSION`).

Run `kubectl retina shell -h` for full documentation and examples.

## Example: Running pwru

To use `pwru` inside the retina shell, you must grant the following Linux capabilities to the container:

* `NET_ADMIN`
* `SYS_ADMIN`

Capability requirements are based on common eBPF tool practices and not directly from the pwru documentation.

Example command to launch a shell with the required capabilities:

```sh
# Pod debugging
kubectl retina shell -n kube-system pod/coredns-57d886c994-8m2ph --capabilities=NET_ADMIN,SYS_ADMIN
```

Once inside the shell, you can run:

```sh
pwru --help
```

Currently only Linux is supported; Windows support will be added in the future.
