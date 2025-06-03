# skopeo inspect docker://mcr.microsoft.com/oss/go/microsoft/golang:1.24.2-2-cbl-mariner2.0 --format "{{.Name}}@{{.Digest}}"
FROM --platform=$BUILDPLATFORM mcr.microsoft.com/oss/go/microsoft/golang@sha256:5341a0010ecff114ee2f11f5eaa4f73b721b54142954041523f3e785d5c4b978 AS builder

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
FROM --platform=$BUILDPLATFORM mcr.microsoft.com/cbl-mariner/distroless/minimal@sha256:db87903c5d4d9d6760e86a274914efd6a3bb5914c0b5a6c6b35350ec297fea4f
WORKDIR /
COPY --from=builder /workspace/kubectl-retina .

