# pinned base images

# skopeo inspect docker://mcr.microsoft.com/oss/go/microsoft/golang:1.26.9-azurelinux3.0 --format "{{.Name}}@{{.Digest}}"
FROM --platform=$BUILDPLATFORM mcr.microsoft.com/oss/go/microsoft/golang@sha256:39876189c354904163b60a5d24e5bcce52e7285a43586cbdeb1bdc243e210d46 AS golang

# skopeo inspect docker://mcr.microsoft.com/azurelinux/base/core:3.0 --format "{{.Name}}@{{.Digest}}"
FROM mcr.microsoft.com/azurelinux/base/core@sha256:7292f2bcf52c3916758f1a45aba6e4dcb93f466027d7786900ea96c2d23c48e2 AS azurelinux-core

# skopeo inspect docker://mcr.microsoft.com/azurelinux/distroless/minimal:3.0 --format "{{.Name}}@{{.Digest}}"
FROM mcr.microsoft.com/azurelinux/distroless/minimal@sha256:792ea6ed971a69bbca863882354d4ff197ca461e0f699d0256653f7886a62d42 AS azurelinux-distroless

# build stages

# Rebuild Hubble so release binaries cannot retain an older Go runtime or x/net.
FROM golang AS hubble-bin
ARG HUBBLE_VERSION=v1.20.2
ARG GOARCH=amd64
ENV GOTOOLCHAIN=local
ENV CGO_ENABLED=0
ENV GOEXPERIMENT=nosystemcrypto
RUN go mod download github.com/cilium/hubble@${HUBBLE_VERSION} && \
    cp -a "$(go env GOMODCACHE)/github.com/cilium/hubble@${HUBBLE_VERSION}" /hubble && \
    chmod -R u+w /hubble
WORKDIR /hubble
RUN go get golang.org/x/net@v0.60.0
RUN --mount=type=cache,target="/root/.cache/go-build" \
    GOOS=linux GOARCH=${GOARCH} go build -mod=mod \
    -ldflags "-s -w -X github.com/cilium/cilium/hubble/pkg.Version=${HUBBLE_VERSION}" \
    -o /out/hubble .

# intermediate go generate stage
FROM golang AS intermediate 
ARG APP_INSIGHTS_ID # set to enable AI telemetry
ARG GOARCH=amd64 # default to amd64
ARG GOOS=linux # default to linux
ENV GOARCH=${GOARCH}
ENV GOOS=${GOOS}
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
    fi
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
COPY --from=tools /lib/ /lib
COPY --from=tools /usr/lib/ /usr/lib
COPY --from=tools /etc/pki/tls/ /etc/pki/tls/
COPY --from=tools /tmp/bin/ /bin
COPY --from=controller-bin /go/bin/retina/controller /retina/controller
COPY --from=controller-bin /go/src/github.com/microsoft/retina/pkg/plugin /go/src/github.com/microsoft/retina/pkg/plugin
COPY --from=capture-bin /go/bin/retina/captureworkload /retina/captureworkload
# Copy Hubble.
COPY --from=hubble-bin /out/hubble /bin/hubble
# Set Hubble server.
ENV HUBBLE_SERVER=unix:///var/run/cilium/hubble.sock
ENTRYPOINT ["./retina/controller"]