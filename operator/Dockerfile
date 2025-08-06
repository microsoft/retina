# skopeo inspect docker://mcr.microsoft.com/oss/go/microsoft/golang:1.24.5-azurelinux3.0 --format "{{.Name}}@{{.Digest}}"
FROM --platform=$BUILDPLATFORM mcr.microsoft.com/oss/go/microsoft/golang@sha256:7f705f63001ad064018c1610455ae1c23e6c8179fbc6cc87bcd5d51d2c4760f6 AS builder

ARG VERSION
ARG APP_INSIGHTS_ID

WORKDIR /workspace
COPY . .

RUN tdnf install -y jq

# Default linux/architecture.
ARG GOOS=linux
ENV GOOS=${GOOS}

ARG GOARCH=amd64
ENV GOARCH=${GOARCH}

RUN make manifests
RUN --mount=type=cache,target="/root/.cache/go-build" \
	GOOS=$GOOS GOARCH=$GOARCH go build \
	-ldflags "-X github.com/microsoft/retina/internal/buildinfo.Version="$VERSION" \
	-X "github.com/microsoft/retina/internal/buildinfo.ApplicationInsightsID"="$APP_INSIGHTS_ID"" \
	-a -o retina-operator operator/main.go

##################### controller #######################
# skopeo inspect docker://mcr.microsoft.com/azurelinux/distroless/minimal:3.0 --format "{{.Name}}@{{.Digest}}"
FROM --platform=$BUILDPLATFORM mcr.microsoft.com/azurelinux/distroless/minimal@sha256:0801b80a0927309572b9adc99bd1813bc680473175f6e8175cd4124d95dbd50c
WORKDIR /
COPY --from=builder /lib /lib
COPY --from=builder /usr/lib/ /usr/lib
COPY --from=builder /workspace/retina-operator .
USER 65532:65532

ENTRYPOINT ["/retina-operator"]
