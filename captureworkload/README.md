# Retina Capture

This directory serves as the entrypoint to Retina Capture: a wrapper to handle lifecyle of packet capture workload

## Build

```bash
 make retina-capture-workload
```

## Test

```bash
HOSTPATH="/tmp/" CAPTURE_NAME="out" CAPTURE_DURATION="10s"  output/linux_amd64/retina/captureworkload
```

## Release

```bash
 make retina-image
 make retina-image-push
```
