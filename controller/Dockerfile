ARG OS_VERSION

# pinned base images

# mcr.microsoft.com/oss/go/microsoft/golang:1.22.3-1-cbl-mariner2.0
FROM --platform=$BUILDPLATFORM mcr.microsoft.com/oss/go/microsoft/golang@sha256:8253def0216b87b2994b7ad689aeec7440f6eb67f981e438071d8d67e36ff69f as golang

# mcr.microsoft.com/cbl-mariner/base/core:2.0
FROM --platform=$TARGETPLATFORM mcr.microsoft.com/cbl-mariner/base/core@sha256:77651116f2e83cf50fddd8a0316945499f8ce6521ff8e94e67539180d1e5975a as mariner-core

# mcr.microsoft.com/cbl-mariner/distroless/minimal:2.0
FROM --platform=$TARGETPLATFORM mcr.microsoft.com/cbl-mariner/distroless/minimal@sha256:63a0a70ceaa1320bc6eb98b81106667d43e46b674731ea8d28e4de1b87e0747f as mariner-distroless

# mcr.microsoft.com/windows/servercore:ltsc2019 
FROM --platform=$TARGETPLATFORM mcr.microsoft.com/windows/servercore@sha256:6fdf140282a2f809dae9b13fe441635867f0a27c33a438771673b8da8f3348a4 as ltsc2019

# mcr.microsoft.com/windows/servercore:ltsc2022
FROM --platform=$TARGETPLATFORM mcr.microsoft.com/windows/servercore@sha256:45952938708fbde6ec0b5b94de68bcdec3f8c838be018536b1e9e5bd95e6b943 as ltsc2022


# build stages

# intermediate go generate stage
FROM golang AS intermediate 
ARG APP_INSIGHTS_ID # set to enable AI telemetry
ARG GOARCH=amd64 # default to amd64
ARG GOOS=linux # default to linux
ENV CGO_ENABLED=0
ENV GOARCH=${GOARCH}
ENV GOOS=${GOOS}
COPY . /go/src/github.com/microsoft/retina
WORKDIR /go/src/github.com/microsoft/retina
RUN if [ "$GOOS" = "linux" ] ; then \
    tdnf install -y clang16 lld16 bpftool libbpf-devel; \
    go generate -x /go/src/github.com/microsoft/retina/pkg/plugin/...; \
    fi

# capture binary
FROM golang AS capture-bin
ARG APP_INSIGHTS_ID # set to enable AI telemetry
ARG GOARCH=amd64 # default to amd64
ARG GOOS=linux # default to linux
ARG VERSION
ENV CGO_ENABLED=0
ENV GOARCH=${GOARCH}
ENV GOOS=${GOOS}
COPY . /go/src/github.com/microsoft/retina 
WORKDIR /go/src/github.com/microsoft/retina
RUN --mount=type=cache,target="/root/.cache/go-build" go build -v -o /go/bin/retina/captureworkload -ldflags "-X main.version="$VERSION" -X main.applicationInsightsID="$APP_INSIGHTS_ID"" captureworkload/main.go


# controller binary
FROM intermediate AS controller-bin
ARG APP_INSIGHTS_ID # set to enable AI telemetry
ARG GOARCH=amd64 # default to amd64
ARG GOOS=linux # default to linux
ARG VERSION
ENV CGO_ENABLED=0
ENV GOARCH=${GOARCH}
ENV GOOS=${GOOS}
RUN --mount=type=cache,target="/root/.cache/go-build" go build -v -o /go/bin/retina/controller -ldflags "-X main.version="$VERSION" -X main.applicationInsightsID="$APP_INSIGHTS_ID"" controller/main.go 


# init binary
FROM intermediate AS init-bin
ARG APP_INSIGHTS_ID # set to enable AI telemetry
ARG GOARCH=amd64 # default to amd64
ARG GOOS=linux # default to linux
ARG VERSION
ENV CGO_ENABLED=0
ENV GOARCH=${GOARCH}
ENV GOOS=${GOOS}
RUN --mount=type=cache,target="/root/.cache/go-build" go build -v -o /go/bin/retina/initretina -ldflags "-X main.version="$VERSION" -X main.applicationInsightsID="$APP_INSIGHTS_ID"" init/retina/main_linux.go


# tools image
FROM mariner-core AS tools
RUN tdnf install -y \
    clang16 \
    iproute \
    iptables \
    tcpdump \
    which
RUN mkdir -p /tmp/bin
RUN arr="clang tcpdump ip ss iptables-legacy iptables-legacy-save iptables-nft iptables-nft-save cp uname" ;\
    for i in $arr; do    \
    cp $(which $i) /tmp/bin;   \
    done


# init final image
FROM mariner-distroless as init
COPY --from=init-bin /go/bin/retina/initretina /retina/initretina
COPY --from=tools /lib/ /lib
COPY --from=tools /usr/lib/ /usr/lib
ENTRYPOINT ["./retina/initretina"]


# agent final image
# mcr.microsoft.com/cbl-mariner/distroless/minimal:2.0
# mcr.microsoft.com/cbl-mariner/distroless/minimal@sha256:63a0a70ceaa1320bc6eb98b81106667d43e46b674731ea8d28e4de1b87e0747f
FROM mariner-distroless as agent
COPY --from=tools /lib/ /lib
COPY --from=tools /usr/lib/ /usr/lib
COPY --from=tools /tmp/bin/ /bin
COPY --from=controller-bin /go/bin/retina/controller /retina/controller
COPY --from=controller-bin /go/src/github.com/microsoft/retina/pkg/plugin /go/src/github.com/microsoft/retina/pkg/plugin
COPY --from=capture-bin /go/bin/retina/captureworkload /retina/captureworkload
ENTRYPOINT ["./retina/controller"]


# agent final image for windows 
FROM ${OS_VERSION} as agent-win
COPY --from=controller-bin /go/src/github.com/microsoft/retina/windows/kubeconfigtemplate.yaml kubeconfigtemplate.yaml
COPY --from=controller-bin /go/src/github.com/microsoft/retina/windows/setkubeconfigpath.ps1 setkubeconfigpath.ps1
COPY --from=controller-bin /go/bin/retina/controller controller.exe
COPY --from=capture-bin /go/bin/retina/captureworkload captureworkload.exe
ADD https://github.com/microsoft/etl2pcapng/releases/download/v1.10.0/etl2pcapng.exe /etl2pcapng.exe
CMD ["controller.exe", "start", "--kubeconfig=.\\kubeconfig"]
