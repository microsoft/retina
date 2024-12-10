ARG OS_VERSION

# pinned base images

# mcr.microsoft.com/oss/go/microsoft/golang:1.23.1-cbl-mariner2.0
FROM --platform=$BUILDPLATFORM mcr.microsoft.com/oss/go/microsoft/golang@sha256:8ac5373de7fde69d08c52e4a1ba40e976a543b3e93196f7fe07c3f91853865f3 AS golang

# mcr.microsoft.com/cbl-mariner/base/core:2.0
FROM --platform=$TARGETPLATFORM mcr.microsoft.com/cbl-mariner/base/core@sha256:77651116f2e83cf50fddd8a0316945499f8ce6521ff8e94e67539180d1e5975a AS mariner-core

# mcr.microsoft.com/cbl-mariner/distroless/minimal:2.0
FROM --platform=$TARGETPLATFORM mcr.microsoft.com/cbl-mariner/distroless/minimal@sha256:db87903c5d4d9d6760e86a274914efd6a3bb5914c0b5a6c6b35350ec297fea4f AS mariner-distroless

# mcr.microsoft.com/windows/servercore:ltsc2019 
FROM --platform=$TARGETPLATFORM mcr.microsoft.com/windows/servercore@sha256:6fdf140282a2f809dae9b13fe441635867f0a27c33a438771673b8da8f3348a4 AS ltsc2019

# mcr.microsoft.com/windows/servercore:ltsc2022
FROM --platform=$TARGETPLATFORM mcr.microsoft.com/windows/servercore@sha256:45952938708fbde6ec0b5b94de68bcdec3f8c838be018536b1e9e5bd95e6b943 AS ltsc2022


# build stages

# intermediate go generate stage
FROM golang AS intermediate 
ARG APP_INSIGHTS_ID # set to enable AI telemetry
ARG GOARCH=amd64 # default to amd64
ARG GOOS=linux # default to linux
ENV GOARCH=${GOARCH}
ENV GOOS=${GOOS}
RUN if [ "$GOOS" = "linux" ] ; then \
      tdnf install -y clang16 lld16 bpftool libbpf-devel; \
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
RUN --mount=type=cache,target="/root/.cache/go-build" go build -v -o /go/bin/retina/controller -ldflags "-X github.com/microsoft/retina/internal/buildinfo.Version="$VERSION" -X github.com/microsoft/retina/internal/buildinfo.ApplicationInsightsID="$APP_INSIGHTS_ID"" controller/main.go 


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
FROM mariner-core AS tools
RUN tdnf install -y \
    clang16 \
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
# Download Hubble
ARG GOARCH=amd64
ENV HUBBLE_ARCH=${GOARCH}
ARG HUBBLE_VERSION=v1.16.3
ENV HUBBLE_VERSION=${HUBBLE_VERSION}
RUN echo "Hubble version: $HUBBLE_VERSION" && \
    wget --no-check-certificate https://github.com/cilium/hubble/releases/download/$HUBBLE_VERSION/hubble-linux-${HUBBLE_ARCH}.tar.gz && \
    wget --no-check-certificate https://github.com/cilium/hubble/releases/download/$HUBBLE_VERSION/hubble-linux-${HUBBLE_ARCH}.tar.gz.sha256sum && \
    sha256sum --check hubble-linux-${HUBBLE_ARCH}.tar.gz.sha256sum && \
    tar xzvfC hubble-linux-${HUBBLE_ARCH}.tar.gz /usr/local && \
    rm hubble-linux-${HUBBLE_ARCH}.tar.gz && rm hubble-linux-${HUBBLE_ARCH}.tar.gz.sha256sum

# init final image
FROM mariner-distroless AS init
COPY --from=init-bin /go/bin/retina/initretina /retina/initretina
COPY --from=tools /lib/ /lib
COPY --from=tools /usr/lib/ /usr/lib
ENTRYPOINT ["./retina/initretina"]


# agent final image
# mcr.microsoft.com/cbl-mariner/distroless/minimal:2.0
# mcr.microsoft.com/cbl-mariner/distroless/minimal@sha256:63a0a70ceaa1320bc6eb98b81106667d43e46b674731ea8d28e4de1b87e0747f
FROM mariner-distroless AS agent
COPY --from=tools /lib/ /lib
COPY --from=tools /usr/lib/ /usr/lib
COPY --from=tools /tmp/bin/ /bin
COPY --from=controller-bin /go/bin/retina/controller /retina/controller
COPY --from=controller-bin /go/src/github.com/microsoft/retina/pkg/plugin /go/src/github.com/microsoft/retina/pkg/plugin
COPY --from=capture-bin /go/bin/retina/captureworkload /retina/captureworkload
# Copy Hubble.
COPY --from=tools /usr/local/hubble /bin/hubble
# Set Hubble server.
ENV HUBBLE_SERVER=unix:///var/run/cilium/hubble.sock
ENTRYPOINT ["./retina/controller"]


# agent final image for windows 
FROM ${OS_VERSION} AS agent-win
COPY --from=controller-bin /go/bin/retina/controller controller.exe
COPY --from=capture-bin /go/bin/retina/captureworkload captureworkload.exe
ADD https://github.com/microsoft/etl2pcapng/releases/download/v1.10.0/etl2pcapng.exe /etl2pcapng.exe
CMD ["controller.exe", "start", "--kubeconfig=.\\kubeconfig"]
