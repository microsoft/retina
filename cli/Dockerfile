# skopeo inspect docker://mcr.microsoft.com/oss/go/microsoft/golang:1.26-azurelinux3.0 --format "{{.Name}}@{{.Digest}}"
FROM --platform=$BUILDPLATFORM mcr.microsoft.com/oss/go/microsoft/golang:1.26-azurelinux3.0@sha256:2de38140ac3783ac72ea4481c51ca5c03943cfa032b1776769a1e185918bd027 AS builder

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
FROM mcr.microsoft.com/azurelinux/base/core:3.0.20261005@sha256:bfd3e44899fe7c17f6fda42a6ef2a322f2178c2dafb88e69fd87675cdcac39ec AS libs
# Keep this pin until the Azure Linux base includes pcre2 >= 10.48-1.azl3; verify both architectures before removing.
RUN tdnf install -y pcre2-10.48-1.azl3 && tdnf clean all

# Target 1: Distroless (secure, minimal)
# skopeo inspect docker://mcr.microsoft.com/azurelinux/distroless/minimal:3.0 --format "{{.Name}}@{{.Digest}}"
# Final image must be $TARGETPLATFORM (default), not $BUILDPLATFORM — the
# kubectl-retina binary is cross-compiled for the target arch and would fail
# to run inside a wrong-arch base image.
FROM mcr.microsoft.com/azurelinux/distroless/minimal:3.0.20260923@sha256:792ea6ed971a69bbca863882354d4ff197ca461e0f699d0256653f7886a62d42 AS distroless-target
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
FROM mcr.microsoft.com/azurelinux/base/core:3.0.20261005@sha256:bfd3e44899fe7c17f6fda42a6ef2a322f2178c2dafb88e69fd87675cdcac39ec AS shell-target
# Keep this pin until the Azure Linux base includes pcre2 >= 10.48-1.azl3; verify both architectures before removing.
RUN tdnf install -y pcre2-10.48-1.azl3 && tdnf clean all
WORKDIR /
COPY --from=builder /workspace/kubectl-retina /bin/kubectl-retina
RUN chmod +x /bin/kubectl-retina

# Default target (distroless for backward compatibility)
FROM distroless-target
