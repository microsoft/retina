# mcr.microsoft.com/oss/go/microsoft/golang:1.23.1-cbl-mariner2.0
FROM --platform=$BUILDPLATFORM mcr.microsoft.com/oss/go/microsoft/golang@sha256:8ac5373de7fde69d08c52e4a1ba40e976a543b3e93196f7fe07c3f91853865f3 AS builder


ARG VERSION
ARG APP_INSIGHTS_ID

WORKDIR /workspace
COPY . .

# Default linux/architecture.
ARG GOOS=linux
ENV GOOS=${GOOS}

ARG GOARCH=amd64
ENV GOARCH=${GOARCH}

RUN --mount=type=cache,target="/root/.cache/go-build" \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags "-X github.com/microsoft/retina/internal/buildinfo.Version="$VERSION" \
    -X "github.com/microsoft/retina/internal/buildinfo.ApplicationInsightsID"="$APP_INSIGHTS_ID"" \
    -a -o kubectl-retina cli/main.go

# mcr.microsoft.com/cbl-mariner/base/core:2.0
FROM --platform=$TARGETPLATFORM mcr.microsoft.com/cbl-mariner/base/core@sha256:77651116f2e83cf50fddd8a0316945499f8ce6521ff8e94e67539180d1e5975a
WORKDIR /
COPY --from=builder /workspace/kubectl-retina .

