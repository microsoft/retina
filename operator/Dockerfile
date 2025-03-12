# skopeo inspect docker://mcr.microsoft.com/oss/go/microsoft/golang:1.24.1-2-cbl-mariner2.0 --format "{{.Name}}@{{.Digest}}"
FROM --platform=$BUILDPLATFORM mcr.microsoft.com/oss/go/microsoft/golang@sha256:00e2221efa6ea3f01b4408432e94ad2cddf5a44fad5882adb7ec5cbcf61a0ec8 AS builder

ARG VERSION
ARG APP_INSIGHTS_ID

WORKDIR /workspace
COPY . .

# Install jq, not sure why this went missing in the 1.24.1-2-cbl-mariner2.0 base go image.
RUN apt-get update && apt-get install -y jq && apt-get clean

# Default linux/architecture.
ARG GOOS=linux
ENV GOOS=${GOOS}

ARG GOARCH=amd64
ENV GOARCH=${GOARCH}

RUN make manifests
RUN --mount=type=cache,target="/root/.cache/go-build" \
	GOOS=linux GOARCH=amd64 go build \
	-ldflags "-X github.com/microsoft/retina/internal/buildinfo.Version="$VERSION" \
	-X "github.com/microsoft/retina/internal/buildinfo.ApplicationInsightsID"="$APP_INSIGHTS_ID"" \
	-a -o retina-operator operator/main.go

##################### controller #######################
# mcr.microsoft.com/cbl-mariner/distroless/minimal:2.0
FROM --platform=$BUILDPLATFORM mcr.microsoft.com/cbl-mariner/distroless/minimal@sha256:db87903c5d4d9d6760e86a274914efd6a3bb5914c0b5a6c6b35350ec297fea4f
WORKDIR /
COPY --from=builder /lib /lib
COPY --from=builder /usr/lib/ /usr/lib
COPY --from=builder /workspace/retina-operator .
USER 65532:65532

ENTRYPOINT ["/retina-operator"]
