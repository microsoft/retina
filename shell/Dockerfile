# skopeo inspect docker://mcr.microsoft.com/oss/go/microsoft/golang:1.26.9-azurelinux3.0 --format "{{.Name}}@{{.Digest}}"
FROM mcr.microsoft.com/oss/go/microsoft/golang@sha256:39876189c354904163b60a5d24e5bcce52e7285a43586cbdeb1bdc243e210d46 AS pwru-builder

ARG GOARCH=amd64
ARG PWRU_TAG=v1.0.12
ENV GOTOOLCHAIN=local
ENV GOFLAGS=-mod=mod
ENV GOEXPERIMENT=nosystemcrypto
RUN tdnf install -y clang flex bison gawk glibc-static
RUN go mod download github.com/cilium/pwru@${PWRU_TAG} && \
    cp -a "$(go env GOMODCACHE)/github.com/cilium/pwru@${PWRU_TAG}" /pwru && \
    chmod -R u+w /pwru && \
    chmod u+x /pwru/libpcap/configure /pwru/libpcap/config.guess /pwru/libpcap/config.sub
WORKDIR /pwru
# Rebuild the embedded Go runtime and dependencies, not just the release archive.
RUN go get golang.org/x/net@v0.60.0
RUN --mount=type=cache,target="/root/.cache/go-build" \
    make TARGET_GOARCH=${GOARCH} VERSION=${PWRU_TAG}

# skopeo inspect docker://mcr.microsoft.com/azurelinux/base/core:3.0 --format "{{.Name}}@{{.Digest}}"
FROM mcr.microsoft.com/azurelinux/base/core@sha256:7292f2bcf52c3916758f1a45aba6e4dcb93f466027d7786900ea96c2d23c48e2

RUN tdnf install -y \
	bind-utils \
	conntrack \
	curl \
	ebtables-legacy \
	ethtool \
	iperf3 \
	iproute \
	ipset \
	iptables \
	iputils \
	jq \
	ldns-utils \
	less \
	net-tools \
	nftables \
	nmap \
	nmap-ncat \
	openssh \
	procps-ng \
	sysstat \
	socat \
	tcpdump \
	wget \
	tar \
	file \
	bpftool \
	bpftrace \
	ig \
	&& tdnf clean all

# Create the entrypoint script to mount debugfs and tracefs
SHELL ["/bin/bash", "-c"]

RUN echo $'#!/bin/bash\n\
if ! mountpoint -q /sys/kernel/debug; then\n\
mount -t debugfs none /sys/kernel/debug\n\
fi\n\
if ! mountpoint -q /sys/kernel/tracing; then\n\
mount -t tracefs tracefs /sys/kernel/tracing\n\
fi\n\
exec "$@"' > /usr/local/bin/entrypoint.sh;

# Set the host root directory for IG
ENV HOST_ROOT=/host

RUN chmod +x /usr/local/bin/entrypoint.sh

# Set the entrypoint
ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]

COPY --from=pwru-builder /pwru/pwru /usr/local/bin/pwru

CMD ["/bin/bash", "-l"]
