# skopeo inspect docker://mcr.microsoft.com/azurelinux/base/core:3.0 --format "{{.Name}}@{{.Digest}}"
FROM mcr.microsoft.com/azurelinux/base/core@sha256:91d58fce1e27dd0b711e569fdc173cfb0aec950ff399ea567723936d785388ba

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
