# mcr.microsoft.com/azurelinux/base/core:3.0.20241203
FROM mcr.microsoft.com/azurelinux/base/core@sha256:07540f424a12aa58f0de61aab38e9670c82f16b35a2ba3e449309596d422109b

# There are a two known issues with Azure Linux 3.0.20241005 that affect this image:
# 1. `iptables-nft` binary is not yet installed, but will be fixed by https://github.com/microsoft/azurelinux/pull/10786
#    Until then, use `nft` to view nftables rules.
# 2. `nslookup` and `bind` print an error "Algorithm not supported by SCOSSL" (but still complete successfully).
#    This will be fixed by https://github.com/microsoft/SymCrypt-OpenSSL/pull/92
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
	net-tools \
	nftables \
	nmap \
	nmap-ncat \
	openssh \
	socat \
	tcpdump \
	wget \
	&& tdnf clean all

CMD ["/bin/bash"]
