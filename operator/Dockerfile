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

RUN make manifests
RUN --mount=type=cache,target="/root/.cache/go-build" \
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
	-ldflags "-X github.com/microsoft/retina/internal/buildinfo.Version="$VERSION" \
	-X "github.com/microsoft/retina/internal/buildinfo.ApplicationInsightsID"="$APP_INSIGHTS_ID"" \
	-a -o retina-operator operator/main.go

##################### controller #######################
# mcr.microsoft.com/cbl-mariner/distroless/minimal:2.0
FROM --platform=$BUILDPLATFORM mcr.microsoft.com/cbl-mariner/distroless/minimal@sha256:63a0a70ceaa1320bc6eb98b81106667d43e46b674731ea8d28e4de1b87e0747f
WORKDIR /
COPY --from=builder /workspace/retina-operator .
USER 65532:65532

ENTRYPOINT ["/retina-operator"]
