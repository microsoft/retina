# skopeo inspect docker://mcr.microsoft.com/oss/go/microsoft/golang:1.26.3-azurelinux3.0 --format "{{.Name}}@{{.Digest}}"
FROM --platform=$BUILDPLATFORM mcr.microsoft.com/oss/go/microsoft/golang:1.26.4-1-azurelinux3.0@sha256:8dbce1c5f7fde5702cdbaa7a352c31477327ca2cec236c8e6c1cae3acb7f936a AS builder

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
FROM mcr.microsoft.com/azurelinux/base/core:3.0.20260616@sha256:1c56f09437dfc2910faad39abaaed336265d246cf183e2adb362d4cb3b881ab6 AS libs

# Target 1: Distroless (secure, minimal)
# skopeo inspect docker://mcr.microsoft.com/azurelinux/distroless/minimal:3.0 --format "{{.Name}}@{{.Digest}}"
FROM --platform=$BUILDPLATFORM mcr.microsoft.com/azurelinux/distroless/minimal:3.0.20260616@sha256:83c9e52e0a0ef97d9d87b8b81da6119f748dbeca4641ae2cb8b11552e2c8f35d AS distroless-target
COPY --from=libs /lib/ /lib
COPY --from=libs /usr/lib/ /usr/lib
COPY --from=libs /etc/pki/tls/ /etc/pki/tls/
WORKDIR /
COPY --from=builder /workspace/kubectl-retina .

# Target 2: Shell-enabled (operational, init container support)
# skopeo inspect docker://mcr.microsoft.com/azurelinux/base/core:3.0 --format "{{.Name}}@{{.Digest}}"
FROM --platform=$BUILDPLATFORM mcr.microsoft.com/azurelinux/base/core:3.0.20260616@sha256:1c56f09437dfc2910faad39abaaed336265d246cf183e2adb362d4cb3b881ab6 AS shell-target
WORKDIR /
COPY --from=builder /workspace/kubectl-retina /bin/kubectl-retina
RUN chmod +x /bin/kubectl-retina

# Default target (distroless for backward compatibility)
FROM distroless-target

