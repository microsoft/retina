# skopeo inspect docker://mcr.microsoft.com/oss/go/microsoft/golang:1.26-azurelinux3.0 --format "{{.Name}}@{{.Digest}}"
FROM --platform=$BUILDPLATFORM mcr.microsoft.com/oss/go/microsoft/golang:1.26-azurelinux3.0@sha256:e8c8da1d9459ae8716a50a0bf01d9c5075039fcbbe7a10c456e476b088778bf1 AS builder

# systemcrypto without cgo — required for arm64 cross-compile from amd64 host.
# Becomes redundant with Go 1.27+ (auto-selected when CGO_ENABLED=0).
ENV GOEXPERIMENT=ms_nocgo_opensslcrypto

ARG VERSION
ARG APP_INSIGHTS_ID

WORKDIR /workspace
COPY . .

# Keep fixed RPM pins until the base image includes them; verify both architectures before removing.
RUN tdnf install -y \
	expat-2.10.0-1.azl3 \
	jq \
	krb5-1.21.3-8.azl3 \
	libxml2-2.11.5-11.azl3 \
	pcre2-10.48-1.azl3 \
	popt-1.19-2.azl3 \
	sqlite-libs-3.44.0-5.azl3 \
	util-linux-2.40.2-6.azl3 \
	zlib-1.3.2-2.azl3

# Default linux/architecture.
ARG GOOS=linux
ENV GOOS=${GOOS}

ARG GOARCH=amd64
ENV GOARCH=${GOARCH}

# make manifests runs controller-gen, a build-time host tool that must be
# compiled for and executed on the build host, not the target arch. Unset the
# cross-compile env for this step so `go tool` doesn't produce an arm64 binary
# and then fail to exec it on the amd64 builder.
RUN env -u GOOS -u GOARCH make manifests
RUN --mount=type=cache,target="/root/.cache/go-build" \
	GOOS=$GOOS GOARCH=$GOARCH go build \
	-ldflags "-X github.com/microsoft/retina/internal/buildinfo.Version="$VERSION" \
	-X "github.com/microsoft/retina/internal/buildinfo.ApplicationInsightsID"="$APP_INSIGHTS_ID"" \
	-a -o retina-operator operator/main.go

##################### controller #######################
# skopeo inspect docker://mcr.microsoft.com/azurelinux/distroless/minimal:3.0 --format "{{.Name}}@{{.Digest}}"
# Final image must be $TARGETPLATFORM (default), not $BUILDPLATFORM — the
# retina-operator binary is cross-compiled for the target arch and would fail
# to run inside a wrong-arch base image.
FROM mcr.microsoft.com/azurelinux/distroless/minimal:3.0.20261116@sha256:acf5853dc3651b91bdbf3aee1e5383e689452f6637bd9bf74c9537771cafa6c2
WORKDIR /
COPY --from=builder /lib /lib
COPY --from=builder /usr/lib/ /usr/lib
COPY --from=builder /etc/pki/tls/ /etc/pki/tls/
COPY --from=builder /workspace/retina-operator .
USER 65532:65532

ENTRYPOINT ["/retina-operator"]
