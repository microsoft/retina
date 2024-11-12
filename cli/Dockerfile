# mcr.microsoft.com/oss/go/microsoft/golang:1.23.1-cbl-mariner2.0
FROM --platform=$BUILDPLATFORM mcr.microsoft.com/oss/go/microsoft/golang@sha256:dcd95cadab21a855894c599c9e26bfb2179aa08e7d7f99c0e8546167712ba6ef AS builder


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

# mcr.microsoft.com/cbl-mariner/distroless/minimal:2.0
FROM --platform=$BUILDPLATFORM mcr.microsoft.com/cbl-mariner/distroless/minimal@sha256:63a0a70ceaa1320bc6eb98b81106667d43e46b674731ea8d28e4de1b87e0747f
WORKDIR /
COPY --from=builder /workspace/kubectl-retina .

