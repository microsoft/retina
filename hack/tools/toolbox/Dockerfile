FROM mcr.microsoft.com/oss/go/microsoft/golang:1.24.5 AS build
ADD . .
WORKDIR /go/toolbox/
RUN CGO_ENABLED=0 GOOS=linux go build -o server .

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
