# mcr.microsoft.com/azurelinux/base/core:3.0.20241203
FROM mcr.microsoft.com/azurelinux/base/core@sha256:07540f424a12aa58f0de61aab38e9670c82f16b35a2ba3e449309596d422109b

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
