FROM mcr.microsoft.com/oss/go/microsoft/golang:1.22 AS builder

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
FROM --platform=$BUILDPLATFORM mcr.microsoft.com/mirror/gcr/distroless/cc-debian11:latest@sha256:b53fbf5f81f4a120a489fedff2092e6fcbeacf7863fce3e45d99cc58dc230ccc AS controller

WORKDIR /
COPY --from=builder /workspace/retina-operator .
USER 65532:65532

ENTRYPOINT ["/retina-operator"]
