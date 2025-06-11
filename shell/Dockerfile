# skopeo inspect docker://mcr.microsoft.com/azurelinux/base/core:3.0 --format "{{.Name}}@{{.Digest}}"
FROM mcr.microsoft.com/azurelinux/base/core@sha256:b46476be0b5c9691ad20f78871819950c01433bdfad81d72c61618f4a6202b25

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
	socat \
	tcpdump \
	wget \
	tar \
	file \
	&& tdnf clean all

# Re-use existing arg from Makefile target "container-docker"
# https://github.com/microsoft/retina/blob/main/Makefile#L224
ARG GOARCH=amd64
ENV ARCH=${GOARCH}
# https://github.com/cilium/pwru/releases
ARG PWRU_TAG="v1.0.9"
ENV PWRU_TAG=${PWRU_TAG}

# Download and extract latest pwru release for the correct architecture (amd64 or arm64)
RUN set -eux; \
    case "$ARCH" in \
        amd64|x86_64) PWRU_ARCH="amd64" ;; \
        arm64|aarch64) PWRU_ARCH="arm64" ;; \
        *) echo "Unsupported arch: $ARCH" && exit 1 ;; \
    esac; \
    PWRU_TAR="pwru-linux-${PWRU_ARCH}.tar.gz"; \
    curl -fL -o /tmp/pwru.tar.gz "https://github.com/cilium/pwru/releases/download/${PWRU_TAG}/${PWRU_TAR}"; \
    tar -xz -C /usr/local/bin -f /tmp/pwru.tar.gz pwru; \
    chmod +x /usr/local/bin/pwru; \
    rm /tmp/pwru.tar.gz; \
    file /usr/local/bin/pwru | grep -q 'ELF'

CMD ["/bin/bash", "-l"]
