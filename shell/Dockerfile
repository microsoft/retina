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
	&& tdnf clean all

CMD ["/bin/bash", "-l"]
