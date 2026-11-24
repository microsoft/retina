# skopeo inspect docker://mcr.microsoft.com/oss/go/microsoft/golang:1.26-azurelinux3.0 --format "{{.Name}}@{{.Digest}}"
FROM --platform=$BUILDPLATFORM mcr.microsoft.com/oss/go/microsoft/golang:1.26-azurelinux3.0@sha256:4b01bf2cc3d952f3f34d46a8b11fbfbe48cd4f06e2f1629b26a749ee4500e68b AS builder

# systemcrypto without cgo — required for arm64 cross-compile from amd64 host.
# Becomes redundant with Go 1.27+ (auto-selected when CGO_ENABLED=0).
ENV GOEXPERIMENT=ms_nocgo_opensslcrypto

ARG VERSION
ARG APP_INSIGHTS_ID
ARG AGENT_IMAGE_NAME="ghcr.io/microsoft/retina/retina-agent"

WORKDIR /workspace
COPY . .

# Default linux/architecture.
ARG GOOS=linux
ENV GOOS=${GOOS}

ARG GOARCH=amd64
ENV GOARCH=${GOARCH}

RUN --mount=type=cache,target="/root/.cache/go-build" \
    go build \
    -ldflags "-X github.com/microsoft/retina/internal/buildinfo.Version="$VERSION" \
    -X "github.com/microsoft/retina/internal/buildinfo.ApplicationInsightsID"="$APP_INSIGHTS_ID" \
    -X "github.com/microsoft/retina/internal/buildinfo.RetinaAgentImageName"="$AGENT_IMAGE_NAME"" \
    -a -o kubectl-retina cli/main.go

# minimal libs stage — provides only the system libraries needed by the CGO-enabled binary,
# without the bloat of the full Go SDK image (python, gcc, systemd, etc.)
# skopeo inspect docker://mcr.microsoft.com/azurelinux/base/core:3.0 --format "{{.Name}}@{{.Digest}}"
FROM mcr.microsoft.com/azurelinux/base/core:3.0.20261116@sha256:3d16d8799f71a78f5bddb3b7cda4a57df1f40ecfa58bac89933010596aa86880 AS libs
# Keep fixed RPM pins until the base image includes them; verify both architectures before removing.
RUN tdnf install -y \
    expat-2.10.0-1.azl3 \
    krb5-1.21.3-8.azl3 \
    pcre2-10.48-1.azl3 \
    popt-1.19-2.azl3 \
    sqlite-libs-3.44.0-5.azl3 \
    zlib-1.3.2-2.azl3 \
    && tdnf clean all

# Target 1: Distroless (secure, minimal)
# skopeo inspect docker://mcr.microsoft.com/azurelinux/distroless/minimal:3.0 --format "{{.Name}}@{{.Digest}}"
# Final image must be $TARGETPLATFORM (default), not $BUILDPLATFORM — the
# kubectl-retina binary is cross-compiled for the target arch and would fail
# to run inside a wrong-arch base image.
FROM mcr.microsoft.com/azurelinux/distroless/minimal:3.0.20261116@sha256:acf5853dc3651b91bdbf3aee1e5383e689452f6637bd9bf74c9537771cafa6c2 AS distroless-target
COPY --from=libs /lib/ /lib
COPY --from=libs /usr/lib/ /usr/lib
COPY --from=libs /etc/pki/tls/ /etc/pki/tls/
WORKDIR /
COPY --from=builder /workspace/kubectl-retina .

# Target 2: Shell-enabled (operational, init container support)
# skopeo inspect docker://mcr.microsoft.com/azurelinux/base/core:3.0 --format "{{.Name}}@{{.Digest}}"
# Final image must be $TARGETPLATFORM (default), not $BUILDPLATFORM — the
# kubectl-retina binary is cross-compiled for the target arch and would fail
# to run inside a wrong-arch base image.
FROM mcr.microsoft.com/azurelinux/base/core:3.0.20261116@sha256:3d16d8799f71a78f5bddb3b7cda4a57df1f40ecfa58bac89933010596aa86880 AS shell-target
# Keep fixed RPM pins until the base image includes them; verify both architectures before removing.
RUN tdnf install -y \
    expat-2.10.0-1.azl3 \
    krb5-1.21.3-8.azl3 \
    pcre2-10.48-1.azl3 \
    popt-1.19-2.azl3 \
    sqlite-libs-3.44.0-5.azl3 \
    zlib-1.3.2-2.azl3 \
    && tdnf clean all
WORKDIR /
COPY --from=builder /workspace/kubectl-retina /bin/kubectl-retina
RUN chmod +x /bin/kubectl-retina

# Default target (distroless for backward compatibility)
FROM distroless-target
