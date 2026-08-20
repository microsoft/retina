# skopeo inspect docker://mcr.microsoft.com/oss/go/microsoft/golang:1.26.3-azurelinux3.0 --format "{{.Name}}@{{.Digest}}"
FROM --platform=$BUILDPLATFORM mcr.microsoft.com/oss/go/microsoft/golang:1.26.5-2-azurelinux3.0@sha256:aa11e321bcccb838417de6cb321513616f334a92f9299195060175f1486a85ea AS builder

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
FROM mcr.microsoft.com/azurelinux/base/core:3.0.20260809@sha256:8bb51342bd5eba915990ab608f91060d502bb7891a2d3d909e0419b932533029 AS libs

# Target 1: Distroless (secure, minimal)
# skopeo inspect docker://mcr.microsoft.com/azurelinux/distroless/minimal:3.0 --format "{{.Name}}@{{.Digest}}"
# Final image must be $TARGETPLATFORM (default), not $BUILDPLATFORM — the
# kubectl-retina binary is cross-compiled for the target arch and would fail
# to run inside a wrong-arch base image.
FROM mcr.microsoft.com/azurelinux/distroless/minimal:3.0.20260809@sha256:4435f90009c17fb750e5518a3f43a24a629ac4c4f8c222b50f6adfe5e0d0bf2d AS distroless-target
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
FROM mcr.microsoft.com/azurelinux/base/core:3.0.20260809@sha256:8bb51342bd5eba915990ab608f91060d502bb7891a2d3d909e0419b932533029 AS shell-target
WORKDIR /
COPY --from=builder /workspace/kubectl-retina /bin/kubectl-retina
RUN chmod +x /bin/kubectl-retina

# Default target (distroless for backward compatibility)
FROM distroless-target

