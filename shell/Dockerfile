# pwru is built from a pinned source commit with this repo's Go builder, so
# its standard library tracks the golang base image instead of the upstream
# release cadence. Upstream publishes binaries only on tags.
# skopeo inspect docker://mcr.microsoft.com/oss/go/microsoft/golang:1.26-azurelinux3.0 --format "{{.Name}}@{{.Digest}}"
FROM mcr.microsoft.com/oss/go/microsoft/golang:1.26-azurelinux3.0@sha256:49a5006e93a8afc634494a0127e858a64f5c11c6c52d2130bf1699e8dc42d725 AS pwru-build
RUN tdnf install -y make git clang llvm gcc glibc-static gawk file flex bison tar diffutils
# https://github.com/cilium/pwru/releases/tag/v1.0.12
ARG PWRU_COMMIT=f1d6cf8898953ac1560401e181f3ac5f0e33b486
RUN git clone https://github.com/cilium/pwru /pwru && git -C /pwru checkout "$PWRU_COMMIT"
WORKDIR /pwru
ARG GOARCH=amd64
RUN set -eux; \
    case "$GOARCH" in \
        amd64) LIBPCAP_ARCH="x86_64-unknown-linux-gnu" ;; \
        arm64) LIBPCAP_ARCH="aarch64-unknown-linux-gnu" ;; \
        *) echo "Unsupported arch: $GOARCH" && exit 1 ;; \
    esac; \
    # GOEXPERIMENT=none disables the Microsoft Go OpenSSL crypto backend,
    # which dlopens libcrypto at startup and crashes inside a statically
    # linked binary. pwru does not need FIPS crypto.
    # CC=clang: the ADO pipeline builds arm64 under QEMU, and gcc cc1
    # crashes there with an internal compiler error. clang does not.
    GOEXPERIMENT=none CC=clang make TARGET_GOARCH="$GOARCH" LIBPCAP_ARCH="$LIBPCAP_ARCH"; \
    file pwru | grep -q 'ELF'; \
    # the binary must start; a broken crypto backend or link crashes here
    ./pwru --version

# skopeo inspect docker://mcr.microsoft.com/azurelinux/base/core:3.0 --format "{{.Name}}@{{.Digest}}"
FROM mcr.microsoft.com/azurelinux/base/core:3.0.20260909@sha256:34a22db497ff34a0f35ca5fc54bd38711d04238a2c1b2f65d35dc9d45dd82584

# Keep this pin until the Azure Linux base includes pcre2 >= 10.48-1.azl3; verify both architectures before removing.
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
	pcre2-10.48-1.azl3 \
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

COPY --from=pwru-build /pwru/pwru /usr/local/bin/pwru
RUN file /usr/local/bin/pwru | grep -q 'ELF' && /usr/local/bin/pwru --version

CMD ["/bin/bash", "-l"]
