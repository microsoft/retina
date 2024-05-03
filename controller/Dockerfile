ARG OS_VERSION=ltsc2019

# intermediate go generate stage
# mcr.microsoft.com/oss/go/microsoft/golang:1.22.2-1-cbl-mariner2.0
# mcr.microsoft.com/oss/go/microsoft/golang@sha256:87e7359c0b4b4e3ca0d4be7fe5099423ec3431d6c44021a84569dba71ac5463e
FROM --platform=$BUILDPLATFORM mcr.microsoft.com/oss/go/microsoft/golang@sha256:87e7359c0b4b4e3ca0d4be7fe5099423ec3431d6c44021a84569dba71ac5463e AS intermediate 
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
# mcr.microsoft.com/oss/go/microsoft/golang:1.22.2-1-cbl-mariner2.0
# mcr.microsoft.com/oss/go/microsoft/golang@sha256:87e7359c0b4b4e3ca0d4be7fe5099423ec3431d6c44021a84569dba71ac5463e
FROM --platform=$BUILDPLATFORM mcr.microsoft.com/oss/go/microsoft/golang@sha256:87e7359c0b4b4e3ca0d4be7fe5099423ec3431d6c44021a84569dba71ac5463e AS capture-bin
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
# mcr.microsoft.com/cbl-mariner/base/core:2.0
# mcr.microsoft.com/cbl-mariner/base/core@sha256:77651116f2e83cf50fddd8a0316945499f8ce6521ff8e94e67539180d1e5975a
FROM  --platform=$TARGETPLATFORM mcr.microsoft.com/cbl-mariner/base/core@sha256:77651116f2e83cf50fddd8a0316945499f8ce6521ff8e94e67539180d1e5975a AS tools
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
# mcr.microsoft.com/cbl-mariner/distroless/minimal:2.0
# mcr.microsoft.com/cbl-mariner/distroless/minimal@sha256:63a0a70ceaa1320bc6eb98b81106667d43e46b674731ea8d28e4de1b87e0747f
FROM --platform=$TARGETPLATFORM mcr.microsoft.com/cbl-mariner/distroless/minimal@sha256:63a0a70ceaa1320bc6eb98b81106667d43e46b674731ea8d28e4de1b87e0747f as init
COPY --from=init-bin /go/bin/retina/initretina /retina/initretina
COPY --from=tools /lib/ /lib
COPY --from=tools /usr/lib/ /usr/lib
ENTRYPOINT ["./retina/initretina"]


# agent final image
# mcr.microsoft.com/cbl-mariner/distroless/minimal:2.0
# mcr.microsoft.com/cbl-mariner/distroless/minimal@sha256:63a0a70ceaa1320bc6eb98b81106667d43e46b674731ea8d28e4de1b87e0747f
FROM --platform=$TARGETPLATFORM mcr.microsoft.com/cbl-mariner/distroless/minimal@sha256:63a0a70ceaa1320bc6eb98b81106667d43e46b674731ea8d28e4de1b87e0747f as agent
COPY --from=tools /lib/ /lib
COPY --from=tools /usr/lib/ /usr/lib
COPY --from=tools /tmp/bin/ /bin
COPY --from=controller-bin /go/bin/retina/controller /retina/controller
COPY --from=controller-bin /go/src/github.com/microsoft/retina/pkg/plugin /go/src/github.com/microsoft/retina/pkg/plugin
COPY --from=capture-bin /go/bin/retina/captureworkload /retina/captureworkload
ENTRYPOINT ["./retina/controller"]


# agent final image for win2019
FROM --platform=$TARGETPLATFORM mcr.microsoft.com/windows/servercore:${OS_VERSION} as agent-win
COPY --from=controller-bin /go/src/github.com/microsoft/retina/windows/kubeconfigtemplate.yaml kubeconfigtemplate.yaml
COPY --from=controller-bin /go/src/github.com/microsoft/retina/windows/setkubeconfigpath.ps1 setkubeconfigpath.ps1
COPY --from=controller-bin /go/bin/retina/controller controller.exe
COPY --from=capture-bin /go/bin/retina/captureworkload captureworkload.exe
ADD https://github.com/microsoft/etl2pcapng/releases/download/v1.10.0/etl2pcapng.exe /etl2pcapng.exe
CMD ["controller.exe", "start", "--kubeconfig=.\\kubeconfig"]
