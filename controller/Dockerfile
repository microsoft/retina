# pinned base images

# skopeo inspect docker://mcr.microsoft.com/oss/go/microsoft/golang:1.26.3-azurelinux3.0 --format "{{.Name}}@{{.Digest}}"
# Pinned to $BUILDPLATFORM so the Go builds cross-compile natively from the
# host arch (fast). The eBPF .o files, which cannot be cleanly cross-compiled
# by bpf2go, are produced in the separate `bpf-gen` stage below (which runs
# at $TARGETPLATFORM — native on same-arch, emulated under QEMU otherwise).
FROM --platform=$BUILDPLATFORM mcr.microsoft.com/oss/go/microsoft/golang:1.26.5-1-azurelinux3.0@sha256:7fd5f6da5a047e6d32f1d51f7de03217b9307edfd0be3235a596c994c81d8344 AS golang

# skopeo inspect docker://mcr.microsoft.com/azurelinux/base/core:3.0 --format "{{.Name}}@{{.Digest}}"
FROM mcr.microsoft.com/azurelinux/base/core:3.0.20260706@sha256:0cdd0c6a200fc2b5d6da711c34228126034bd428650b43dfb7e378214e6f2d32 AS azurelinux-core

# skopeo inspect docker://mcr.microsoft.com/azurelinux/distroless/minimal:3.0 --format "{{.Name}}@{{.Digest}}"
FROM mcr.microsoft.com/azurelinux/distroless/minimal:3.0.20260706@sha256:576d9769c0146cbf0cf7946bacf536c5758464c29eadfa03ef5090ae708e641f AS azurelinux-distroless

# build stages

# bpf-gen stage: runs at $TARGETPLATFORM (native for same-arch builds, emulated
# via QEMU for cross-arch builds). Only responsible for producing correct eBPF
# .o files via bpf2go/clang — those require native target-arch execution and
# cannot be cross-compiled from an amd64 host cleanly. Isolating this work in a
# small dedicated stage keeps the rest of the build fast at $BUILDPLATFORM.
FROM mcr.microsoft.com/oss/go/microsoft/golang:1.26.5-1-azurelinux3.0@sha256:7fd5f6da5a047e6d32f1d51f7de03217b9307edfd0be3235a596c994c81d8344 AS bpf-gen
ARG GOOS=linux
ARG GOARCH=amd64
ENV GOOS=${GOOS}
ENV GOARCH=${GOARCH}
RUN if [ "$GOOS" = "linux" ] ; then \
    tdnf install -y clang lld bpftool libbpf-devel; \
    fi
COPY ./pkg/plugin /go/src/github.com/microsoft/retina/pkg/plugin
WORKDIR /go/src/github.com/microsoft/retina
RUN if [ "$GOOS" = "linux" ] ; then \
    go mod init github.com/microsoft/retina; \
    go generate -skip "mockgen" -x /go/src/github.com/microsoft/retina/pkg/plugin/...; \
    tar czf /gen.tar.gz ./pkg/plugin; \
    rm go.mod; \
    else \
    tar czf /gen.tar.gz --files-from /dev/null; \
    fi

# intermediate go generate stage
FROM golang AS intermediate 
# systemcrypto without cgo — required for arm64 cross-compile from amd64 host
# (Go auto-disables cgo when cross-compiling, and systemcrypto in Go <=1.26
# requires cgo unless this experiment is set). Becomes redundant with Go 1.27+.
ENV GOEXPERIMENT=ms_nocgo_opensslcrypto
ARG APP_INSIGHTS_ID # set to enable AI telemetry
ARG GOARCH=amd64 # default to amd64
ARG GOOS=linux # default to linux
ENV GOARCH=${GOARCH}
ENV GOOS=${GOOS}
WORKDIR /go/src/github.com/microsoft/retina
# eBPF .o files were built in the bpf-gen stage (at $TARGETPLATFORM). Import
# the tarball here and extract after `COPY . .` so bpf-gen's outputs replace
# the zero-byte placeholders checked into the repo.
COPY --from=bpf-gen /gen.tar.gz /gen.tar.gz
COPY ./go.mod ./go.sum ./
RUN go mod download
COPY . .
RUN if [ "$GOOS" = "linux" ] ; then \
    rm -rf ./pkg/plugin && tar xvf /gen.tar.gz ./pkg/plugin; \
    fi

# capture binary
FROM intermediate AS capture-bin
ARG APP_INSIGHTS_ID # set to enable AI telemetry
ARG GOARCH=amd64 # default to amd64
ARG GOOS=linux # default to linux
ARG VERSION
ENV GOARCH=${GOARCH}
ENV GOOS=${GOOS}
RUN --mount=type=cache,target="/root/.cache/go-build" go build -v -o /go/bin/retina/captureworkload -ldflags "-X github.com/microsoft/retina/internal/buildinfo.Version="$VERSION" -X github.com/microsoft/retina/internal/buildinfo.ApplicationInsightsID="$APP_INSIGHTS_ID"" captureworkload/main.go

# controller binary
FROM intermediate AS controller-bin
ARG APP_INSIGHTS_ID # set to enable AI telemetry
ARG GOARCH=amd64 # default to amd64
ARG GOOS=linux # default to linux
ARG VERSION
ENV GOARCH=${GOARCH}
ENV GOOS=${GOOS}
RUN --mount=type=cache,target="/root/.cache/go-build" go build -x -v -o /go/bin/retina/controller -ldflags "-X github.com/microsoft/retina/internal/buildinfo.Version="$VERSION" -X github.com/microsoft/retina/internal/buildinfo.ApplicationInsightsID="$APP_INSIGHTS_ID"" controller/main.go 

# init binary
FROM intermediate AS init-bin
ARG APP_INSIGHTS_ID # set to enable AI telemetry
ARG GOARCH=amd64 # default to amd64
ARG GOOS=linux # default to linux
ARG VERSION
ENV GOARCH=${GOARCH}
ENV GOOS=${GOOS}
RUN --mount=type=cache,target="/root/.cache/go-build" go build -v -o /go/bin/retina/initretina -ldflags "-X github.com/microsoft/retina/internal/buildinfo.Version="$VERSION" -X github.com/microsoft/retina/internal/buildinfo.ApplicationInsightsID="$APP_INSIGHTS_ID"" init/retina/main_linux.go

# tools image
FROM azurelinux-core AS tools
RUN tdnf install -y \
    clang \
    bpftool \
    iproute \
    iptables \
    tcpdump \
    which \
    wget \
    gnupg2 \
    ca-certificates \
    tar
RUN mkdir -p /tmp/bin
RUN arr="clang tcpdump ip ss iptables-legacy iptables-legacy-save iptables-nft iptables-nft-save cp uname" ;\
    for i in $arr; do    \
    cp $(which $i) /tmp/bin;   \
    done
RUN mkdir -p /tmp/init-bin
RUN cp $(which bpftool) /tmp/init-bin
# Download Hubble
ARG GOARCH=amd64
ENV HUBBLE_ARCH=${GOARCH}
# ARG HUBBLE_VERSION may be modified via the update-hubble GitHub Action
ARG HUBBLE_VERSION=v1.19.4
ENV HUBBLE_VERSION=${HUBBLE_VERSION}
RUN echo "Hubble version: $HUBBLE_VERSION" && \
    wget --no-check-certificate https://github.com/cilium/hubble/releases/download/$HUBBLE_VERSION/hubble-linux-${HUBBLE_ARCH}.tar.gz && \
    wget --no-check-certificate https://github.com/cilium/hubble/releases/download/$HUBBLE_VERSION/hubble-linux-${HUBBLE_ARCH}.tar.gz.sha256sum && \
    sha256sum --check hubble-linux-${HUBBLE_ARCH}.tar.gz.sha256sum && \
    tar xzvfC hubble-linux-${HUBBLE_ARCH}.tar.gz /usr/local && \
    rm hubble-linux-${HUBBLE_ARCH}.tar.gz && rm hubble-linux-${HUBBLE_ARCH}.tar.gz.sha256sum

# init final image
FROM azurelinux-distroless AS init
COPY --from=init-bin /go/bin/retina/initretina /retina/initretina
COPY --from=tools /lib/ /lib
COPY --from=tools /usr/lib/ /usr/lib
COPY --from=tools /etc/pki/tls/ /etc/pki/tls/
COPY --from=tools /tmp/init-bin/ /bin/
ENTRYPOINT ["./retina/initretina"]

# agent final image
# mcr.microsoft.com/azurelinux/distroless/minimal:3.0
# mcr.microsoft.com/azurelinux/distroless/minimal@sha256:5a66f9f16ac675db2a8229dac72d83811b73b502d6ad192d8b374c7f3be498af
FROM azurelinux-distroless AS agent
COPY --from=tools /lib/ /lib
COPY --from=tools /usr/lib/ /usr/lib
COPY --from=tools /etc/pki/tls/ /etc/pki/tls/
COPY --from=tools /tmp/bin/ /bin
COPY --from=controller-bin /go/bin/retina/controller /retina/controller
COPY --from=controller-bin /go/src/github.com/microsoft/retina/pkg/plugin /go/src/github.com/microsoft/retina/pkg/plugin
COPY --from=capture-bin /go/bin/retina/captureworkload /retina/captureworkload
# Copy Hubble.
COPY --from=tools /usr/local/hubble /bin/hubble
# Set Hubble server.
ENV HUBBLE_SERVER=unix:///var/run/cilium/hubble.sock
ENTRYPOINT ["./retina/controller"]