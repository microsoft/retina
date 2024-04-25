ARG OS_VERSION=ltsc2019

# capture binary
FROM --platform=$BUILDPLATFORM mcr.microsoft.com/oss/go/microsoft/golang:1.22 AS capture-bin
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
FROM --platform=$BUILDPLATFORM mcr.microsoft.com/oss/go/microsoft/golang:1.22 AS controller-bin
ARG APP_INSIGHTS_ID # set to enable AI telemetry
ARG GOARCH=amd64 # default to amd64
ARG GOOS=linux # default to linux
ARG VERSION
ENV CGO_ENABLED=0
ENV GOARCH=${GOARCH}
ENV GOOS=${GOOS}
COPY . /go/src/github.com/microsoft/retina 
WORKDIR /go/src/github.com/microsoft/retina
RUN --mount=type=cache,target="/root/.cache/go-build" go build -v -o /go/bin/retina/controller -ldflags "-X main.version="$VERSION" -X main.applicationInsightsID="$APP_INSIGHTS_ID"" controller/main.go 


# init binary
FROM --platform=$BUILDPLATFORM mcr.microsoft.com/oss/go/microsoft/golang:1.22 AS init-bin
ARG APP_INSIGHTS_ID # set to enable AI telemetry
ARG GOARCH=amd64 # default to amd64
ARG GOOS=linux # default to linux
ARG VERSION
ENV CGO_ENABLED=0
ENV GOARCH=${GOARCH}
ENV GOOS=${GOOS}
COPY . /go/src/github.com/microsoft/retina 
WORKDIR /go/src/github.com/microsoft/retina
RUN --mount=type=cache,target="/root/.cache/go-build" go build -v -o /go/bin/retina/initretina -ldflags "-X main.version="$VERSION" -X main.applicationInsightsID="$APP_INSIGHTS_ID"" init/retina/main_linux.go


# tools image
FROM  --platform=$TARGETPLATFORM mcr.microsoft.com/mirror/docker/library/debian:bookworm@sha256:1aadfee8d292f64b045adb830f8a58bfacc15789ae5f489a0fedcd517a862cb9 AS tools
RUN apt-get update && \
    apt-get install -y \
    apt-file \
    clang \
    curl \
    gnupg2 \
    iproute2 \
    iptables \
    tcpdump 

RUN mkdir -p /tmp/bin
RUN arr="clang tcpdump ip ss iptables-legacy iptables-legacy-save iptables-nft iptables-nft-save cp uname" ;\
    for i in $arr; do    \
    cp $(which $i) /tmp/bin;   \
    done


# init final image
FROM --platform=$TARGETPLATFORM gcr.io/distroless/cc-debian12:debug as init
COPY --from=init-bin /go/bin/retina/initretina /retina/initretina
COPY --from=tools /lib/ /lib
COPY --from=tools /usr/lib/ /usr/lib
COPY --from=tools /bin/mount /bin/mount
ENTRYPOINT ["./retina/initretina"]


# agent final image
FROM --platform=$TARGETPLATFORM gcr.io/distroless/cc-debian12:debug as agent
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
