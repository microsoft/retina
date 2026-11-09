# skopeo inspect docker://mcr.microsoft.com/oss/go/microsoft/golang:1.26-azurelinux3.0 --format "{{.Name}}@{{.Digest}}"
FROM --platform=$BUILDPLATFORM mcr.microsoft.com/oss/go/microsoft/golang:1.26-azurelinux3.0@sha256:da5a59d8b7462abfdb98bdf47ec8bfb677d9f1cf8afac71dce9a91cbcecd003b AS builder

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
FROM mcr.microsoft.com/azurelinux/base/core:3.0.20261027@sha256:88fc2b4a425801b3b4d8fe5342e24fb6134669fb718ec970340bdd36dee50c35 AS libs
# Keep fixed RPM pins until the base image includes them; verify both architectures before removing.
RUN tdnf install -y \
    krb5-1.21.3-6.azl3 \
    pcre2-10.48-1.azl3 \
    popt-1.19-2.azl3 \
    zlib-1.3.2-2.azl3 \
    && tdnf clean all

# Target 1: Distroless (secure, minimal)
# skopeo inspect docker://mcr.microsoft.com/azurelinux/distroless/minimal:3.0 --format "{{.Name}}@{{.Digest}}"
# Final image must be $TARGETPLATFORM (default), not $BUILDPLATFORM — the
# kubectl-retina binary is cross-compiled for the target arch and would fail
# to run inside a wrong-arch base image.
FROM mcr.microsoft.com/azurelinux/distroless/minimal:3.0.20261027@sha256:ca4f1b9b5d4bb21445fbf5cf7288063ea477a9fa88affa4b13757245e3214914 AS distroless-target
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
FROM mcr.microsoft.com/azurelinux/base/core:3.0.20261027@sha256:88fc2b4a425801b3b4d8fe5342e24fb6134669fb718ec970340bdd36dee50c35 AS shell-target
# Keep fixed RPM pins until the base image includes them; verify both architectures before removing.
RUN tdnf install -y \
    krb5-1.21.3-6.azl3 \
    pcre2-10.48-1.azl3 \
    popt-1.19-2.azl3 \
    zlib-1.3.2-2.azl3 \
    && tdnf clean all
WORKDIR /
COPY --from=builder /workspace/kubectl-retina /bin/kubectl-retina
RUN chmod +x /bin/kubectl-retina

# Default target (distroless for backward compatibility)
FROM distroless-target
