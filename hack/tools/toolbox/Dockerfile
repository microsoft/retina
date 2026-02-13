# skopeo inspect docker://mcr.microsoft.com/oss/go/microsoft/golang:1.26.0 --format "{{.Name}}@{{.Digest}}"
FROM mcr.microsoft.com/oss/go/microsoft/golang@sha256:58360a7ce64a2cf2e4a4315979c48ded5bb8d30e984ea2ae8e17d5660404a69f AS build
ADD . .
WORKDIR /go/toolbox/
RUN GOOS=linux go build -o server .

FROM mcr.microsoft.com/mirror/docker/library/ubuntu:24.04
RUN apt-get update
RUN apt-get install -y \
    axel \
    bridge-utils \
    bpftrace \
    curl \
    dnsutils \
    ebtables \
    git \
    iproute2 \
    iptables \
    iputils-ping \
    iperf3 \
    jq \
    linux-tools-common \
    linux-tools-generic \
    nano \
    ncat \
    net-tools \
    netperf \
    nmap \
    python3 \
    ssh \
    sudo \
    tcpdump \
    traceroute \
    unzip \
    vim \
    wget

RUN curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl"
RUN curl -LO "https://dl.k8s.io/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl.sha256"
RUN install -o root -g root -m 0755 kubectl /usr/local/bin/kubectl
RUN ln -s /usr/local/bin/kubectl /usr/local/bin/k && rm ./kube*

RUN wget https://github.com/microsoft/ethr/releases/latest/download/ethr_linux.zip
RUN unzip ethr_linux.zip

COPY --from=build /go/toolbox/server .
CMD ["./server"]
