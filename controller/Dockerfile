# Only applicable for windows images
ARG OS_VERSION=ltsc2022
# pinned base images

# skopeo inspect docker://mcr.microsoft.com/oss/go/microsoft/golang:1.24.5-azurelinux3.0 --format "{{.Name}}@{{.Digest}}"
FROM --platform=$BUILDPLATFORM mcr.microsoft.com/oss/go/microsoft/golang@sha256:7f705f63001ad064018c1610455ae1c23e6c8179fbc6cc87bcd5d51d2c4760f6 AS golang

# skopeo inspect docker://mcr.microsoft.com/azurelinux/base/core:3.0 --format "{{.Name}}@{{.Digest}}"
FROM mcr.microsoft.com/azurelinux/base/core@sha256:9948138108a3d69f1dae62104599ac03132225c3b7a5ac57b85a214629c8567d AS azurelinux-core

# skopeo inspect docker://mcr.microsoft.com/azurelinux/distroless/minimal:3.0 --format "{{.Name}}@{{.Digest}}"
FROM mcr.microsoft.com/azurelinux/distroless/minimal@sha256:0801b80a0927309572b9adc99bd1813bc680473175f6e8175cd4124d95dbd50c AS azurelinux-distroless

# skopeo inspect docker://mcr.microsoft.com/windows/servercore:ltsc2019  --override-os windows --format "{{.Name}}@{{.Digest}}"
FROM mcr.microsoft.com/windows/servercore@sha256:2a7cfebaed9241227ad68b1fc7cb764867ea1c56624ece03f926eb8bdf0c998f AS ltsc2019

# skopeo inspect docker://mcr.microsoft.com/windows/servercore:ltsc2022  --override-os windows --format "{{.Name}}@{{.Digest}}"
FROM mcr.microsoft.com/windows/servercore@sha256:3281482945016cdaefbe417edd8338de8119e077b6941f74e78b050da1b7bd97 AS ltsc2022

# build stages

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
# ARG HUBBLE_VERSION may be modified via the update-hubble GitHub Action
ARG HUBBLE_VERSION=v1.17.5
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
ENTRYPOINT ["./retina/initretina"]


# agent final image
# mcr.microsoft.com/azurelinux/distroless/minimal:3.0
# mcr.microsoft.com/azurelinux/distroless/minimal@sha256:0801b80a0927309572b9adc99bd1813bc680473175f6e8175cd4124d95dbd50c
FROM azurelinux-distroless AS agent
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
COPY --from=controller-bin /go/src/github.com/microsoft/retina/windows/kubeconfigtemplate.yaml kubeconfigtemplate.yaml
COPY --from=controller-bin /go/src/github.com/microsoft/retina/windows/setkubeconfigpath.ps1 setkubeconfigpath.ps1
COPY --from=controller-bin /go/bin/retina/controller controller.exe
COPY --from=capture-bin /go/bin/retina/captureworkload captureworkload.exe
ADD https://github.com/microsoft/etl2pcapng/releases/download/v1.10.0/etl2pcapng.exe /etl2pcapng.exe
CMD ["controller.exe", "start", "--kubeconfig=.\\kubeconfig"]
