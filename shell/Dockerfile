# pwru is built from a pinned source commit with this repo's Go builder, so
# its standard library tracks the golang base image instead of the upstream
# release cadence. Upstream publishes binaries only on tags.
# skopeo inspect docker://mcr.microsoft.com/oss/go/microsoft/golang:1.26-azurelinux3.0 --format "{{.Name}}@{{.Digest}}"
FROM mcr.microsoft.com/oss/go/microsoft/golang:1.26-azurelinux3.0@sha256:0fd0eb0f8786e0b8c9fff71b12a48d112e6392ce35c84f72650f185f6722977d AS pwru-build
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
FROM mcr.microsoft.com/azurelinux/base/core:3.0.20260825@sha256:daa1142fc6b44e27c8112ec6b4c2d579ddb9bc6b3747504e666010a45a51faa4

RUN tdnf install -y \
	bind-utils \
	conntrack \
	cri-tools \
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
	nano \
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
	&& tdnf clean all \
	# cri-tools also installs critest, a CRI conformance suite binary as large
	# as crictl itself and with no use in a debug shell. Remove it in the same
	# layer so the image does not store it.
	&& rm /usr/bin/critest

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

# Point crictl at the containerd socket, which is exposed under /run when the
# shell is started with --mount-host-filesystem. Override with
# `crictl --runtime-endpoint` on clusters running a different CRI implementation.
RUN echo $'runtime-endpoint: unix:///run/containerd/containerd.sock\n\
image-endpoint: unix:///run/containerd/containerd.sock\n\
timeout: 10' > /etc/crictl.yaml

# crictl logs opens the kubelet's log files at /var/log/pods itself; the CRI
# API does not stream log content. The files exist only on the host, so
# resolve the path through the read-only /host mount. The link dangles
# harmlessly when --mount-host-filesystem is not set.
RUN ln -s /host/var/log/pods /var/log/pods

# Set the entrypoint
ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]

COPY --from=pwru-build /pwru/pwru /usr/local/bin/pwru
RUN file /usr/local/bin/pwru | grep -q 'ELF' && /usr/local/bin/pwru --version

CMD ["/bin/bash", "-l"]
