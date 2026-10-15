# skopeo inspect docker://mcr.microsoft.com/oss/go/microsoft/golang:1.26.9-azurelinux3.0 --format "{{.Name}}@{{.Digest}}"
FROM --platform=$BUILDPLATFORM mcr.microsoft.com/oss/go/microsoft/golang@sha256:39876189c354904163b60a5d24e5bcce52e7285a43586cbdeb1bdc243e210d46 AS builder

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
FROM mcr.microsoft.com/azurelinux/base/core@sha256:7292f2bcf52c3916758f1a45aba6e4dcb93f466027d7786900ea96c2d23c48e2 AS libs

# Target 1: Distroless (secure, minimal)
# skopeo inspect docker://mcr.microsoft.com/azurelinux/distroless/minimal:3.0 --format "{{.Name}}@{{.Digest}}"
FROM --platform=$BUILDPLATFORM mcr.microsoft.com/azurelinux/distroless/minimal@sha256:792ea6ed971a69bbca863882354d4ff197ca461e0f699d0256653f7886a62d42 AS distroless-target
COPY --from=libs /lib/ /lib
COPY --from=libs /usr/lib/ /usr/lib
COPY --from=libs /etc/pki/tls/ /etc/pki/tls/
WORKDIR /
COPY --from=builder /workspace/kubectl-retina .

# Target 2: Shell-enabled (operational, init container support)
# skopeo inspect docker://mcr.microsoft.com/azurelinux/base/core:3.0 --format "{{.Name}}@{{.Digest}}"
FROM --platform=$BUILDPLATFORM mcr.microsoft.com/azurelinux/base/core@sha256:7292f2bcf52c3916758f1a45aba6e4dcb93f466027d7786900ea96c2d23c48e2 AS shell-target
WORKDIR /
COPY --from=builder /workspace/kubectl-retina /bin/kubectl-retina
RUN chmod +x /bin/kubectl-retina

# Default target (distroless for backward compatibility)
FROM distroless-target
