# pinned base images

# skopeo inspect docker://mcr.microsoft.com/oss/go/microsoft/golang:1.26-azurelinux3.0 --format "{{.Name}}@{{.Digest}}"
# Pinned to $BUILDPLATFORM so the Go builds cross-compile natively from the
# host arch (fast). The eBPF .o files, which cannot be cleanly cross-compiled
# by bpf2go, are produced in the separate `bpf-gen` stage below (which runs
# at $TARGETPLATFORM — native on same-arch, emulated under QEMU otherwise).
FROM --platform=$BUILDPLATFORM mcr.microsoft.com/oss/go/microsoft/golang:1.26-azurelinux3.0@sha256:d104fef62900c2ca101f52568f43e0e6b6499b60be324fdd4d2ad5bae4bc3203 AS golang

# skopeo inspect docker://mcr.microsoft.com/azurelinux/base/core:3.0 --format "{{.Name}}@{{.Digest}}"
FROM mcr.microsoft.com/azurelinux/base/core:3.0.20261022@sha256:bd88359666e3f20296e7fe639a480dfd3c3c8eba299b21f26e4560353e21a656 AS azurelinux-core

# skopeo inspect docker://mcr.microsoft.com/azurelinux/distroless/minimal:3.0 --format "{{.Name}}@{{.Digest}}"
FROM mcr.microsoft.com/azurelinux/distroless/minimal:3.0.20261022@sha256:c91e907c537ad861cb90bc18aece90479b0754b090afd4eb8722dd15477dc9e8 AS azurelinux-distroless

# build stages

# bpf-gen stage: runs at $TARGETPLATFORM (native for same-arch builds, emulated
# via QEMU for cross-arch builds). Only responsible for producing correct eBPF
# .o files via bpf2go/clang — those require native target-arch execution and
# cannot be cross-compiled from an amd64 host cleanly. Isolating this work in a
# small dedicated stage keeps the rest of the build fast at $BUILDPLATFORM.
FROM mcr.microsoft.com/oss/go/microsoft/golang:1.26-azurelinux3.0@sha256:d104fef62900c2ca101f52568f43e0e6b6499b60be324fdd4d2ad5bae4bc3203 AS bpf-gen
ARG GOOS=linux
ARG GOARCH=amd64
ENV GOOS=${GOOS}
ENV GOARCH=${GOARCH}
ENV CGO_ENABLED=0
ENV GOEXPERIMENT=ms_nocgo_opensslcrypto
RUN if [ "$GOOS" = "linux" ] ; then \
    tdnf install -y clang lld bpftool libbpf-devel; \
    fi
COPY ./pkg/plugin /go/src/github.com/microsoft/retina/pkg/plugin
WORKDIR /go/src/github.com/microsoft/retina
# Sanity-check every arch-specific BPF object bpf2go produced is non-empty.
RUN set -eu; \
    if [ "$GOOS" = "linux" ] ; then \
    go mod init github.com/microsoft/retina; \
    go generate -skip "mockgen" -x /go/src/github.com/microsoft/retina/pkg/plugin/...; \
    if [ "$GOARCH" = "amd64" ]; then bpf_arch=x86; else bpf_arch="$GOARCH"; fi; \
    objects="$(find ./pkg/plugin -type f -name "*_bpfel_${bpf_arch}.o" -print)"; \
    test -n "$objects" || { echo "no BPF objects generated for ${bpf_arch}" >&2; exit 1; }; \
    empty_objects="$(find ./pkg/plugin -type f -name "*_bpfel_${bpf_arch}.o" ! -size +0c -print)"; \
    test -z "$empty_objects" || { printf 'empty BPF objects:\n%s\n' "$empty_objects" >&2; exit 1; }; \
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

# Hubble CLI binary built from the Cilium source pinned in go.mod.
FROM golang AS hubble-bin
ARG GOARCH=amd64
ENV GOARCH=${GOARCH}
ENV GOEXPERIMENT=ms_nocgo_opensslcrypto
WORKDIR /go/src/github.com/microsoft/retina
COPY ./go.mod ./go.sum ./
RUN go mod download
COPY ./cmd/hubble-cli ./cmd/hubble-cli
RUN CILIUM_VERSION="$(go list -m -f '{{.Version}}' github.com/cilium/cilium)" && \
    test -n "$CILIUM_VERSION" && \
    CGO_ENABLED=0 go build \
    -trimpath \
    -tags=osusergo \
    -ldflags="-s -w -X github.com/cilium/cilium/hubble/pkg.Version=${CILIUM_VERSION}" \
    -o /go/bin/hubble \
    ./cmd/hubble-cli

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
# Keep this pin until the Azure Linux base includes pcre2 >= 10.48-1.azl3; verify both architectures before removing.
RUN tdnf install -y \
    clang \
    bpftool \
    iproute \
    iptables \
    pcre2-10.48-1.azl3 \
    tcpdump \
    which \
    ca-certificates
RUN mkdir -p /tmp/bin
RUN arr="clang tcpdump ip ss iptables-legacy iptables-legacy-save iptables-nft iptables-nft-save cp uname" ;\
    for i in $arr; do    \
    cp $(which $i) /tmp/bin;   \
    done
RUN mkdir -p /tmp/init-bin
RUN cp $(which bpftool) /tmp/init-bin

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
# mcr.microsoft.com/azurelinux/distroless/minimal@sha256:792ea6ed971a69bbca863882354d4ff197ca461e0f699d0256653f7886a62d42
FROM azurelinux-distroless AS agent
ENV HUBBLE_SERVER=unix:///var/run/cilium/hubble.sock
COPY --from=tools /lib/ /lib
COPY --from=tools /usr/lib/ /usr/lib
COPY --from=tools /etc/pki/tls/ /etc/pki/tls/
COPY --from=tools /tmp/bin/ /bin
COPY --from=hubble-bin /go/bin/hubble /bin/hubble
COPY --from=controller-bin /go/bin/retina/controller /retina/controller
COPY --from=controller-bin /go/src/github.com/microsoft/retina/pkg/plugin /go/src/github.com/microsoft/retina/pkg/plugin
COPY --from=capture-bin /go/bin/retina/captureworkload /retina/captureworkload
ENTRYPOINT ["./retina/controller"]
