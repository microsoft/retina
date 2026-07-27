# skopeo inspect docker://mcr.microsoft.com/oss/go/microsoft/golang:1.26.5-azurelinux3.0 --format "{{.Name}}@{{.Digest}}"
FROM --platform=$BUILDPLATFORM mcr.microsoft.com/oss/go/microsoft/golang:1.26.5-azurelinux3.0@sha256:1c77c1cbb5de52db3f119fe2efe7a938e734c08196bbe3ad94b3bdadbab926f9 AS builder

# enable no-cgo OpenSSL crypto for arm64 cross-compile (Go <=1.26)
ENV GOEXPERIMENT=ms_nocgo_opensslcrypto

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

# controller-gen is a host tool; run it natively (unset cross-compile env) so it isn't built for arm64 then exec-failed on the amd64 builder
RUN env -u GOOS -u GOARCH make manifests
RUN --mount=type=cache,target="/root/.cache/go-build" \
	GOOS=$GOOS GOARCH=$GOARCH go build \
	-ldflags "-X github.com/microsoft/retina/internal/buildinfo.Version="$VERSION" \
	-X "github.com/microsoft/retina/internal/buildinfo.ApplicationInsightsID"="$APP_INSIGHTS_ID"" \
	-a -o retina-operator operator/main.go

##################### controller #######################
# skopeo inspect docker://mcr.microsoft.com/azurelinux/distroless/minimal:3.0 --format "{{.Name}}@{{.Digest}}"
FROM mcr.microsoft.com/azurelinux/distroless/minimal:3.0@sha256:5a66f9f16ac675db2a8229dac72d83811b73b502d6ad192d8b374c7f3be498af
WORKDIR /
COPY --from=builder /lib /lib
COPY --from=builder /usr/lib/ /usr/lib
COPY --from=builder /workspace/retina-operator .
USER 65532:65532

ENTRYPOINT ["/retina-operator"]
