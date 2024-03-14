# CLI Setup

Currently, Retina CLI only supports Linux.

For CLI usage, see [Capture with Retina CLI](../captures/cli.md).

## Option 1: Download from Release

Download `kubectl-retina` from the latest [Retina release](https://github.com/microsoft/retina/releases).
Feel free to move the binary to `/usr/local/bin/`, or add it to your `PATH` otherwise.

## Option 2: Build from source

Clone the Retina repo and execute:

```shell
make install-kubectl-retina
```

Requirements:

- go 1.21 or newer
- GNU make
