# Stage: Build binary
FROM mcr.microsoft.com/oss/go/microsoft/golang:1.21 AS builder
LABEL Name=retina-builder Version=0.0.1

RUN apt-get update &&\
    apt-get -y install lsb-release wget software-properties-common gnupg file git make

RUN wget -O - https://apt.llvm.org/llvm-snapshot.gpg.key | apt-key add -
RUN add-apt-repository "deb http://apt.llvm.org/bullseye/ llvm-toolchain-bullseye-14 main"
RUN apt-get update

RUN apt-get install -y clang-14 lldb-14 lld-14 clangd-14 man-db
RUN apt-get install -y bpftool libbpf-dev

RUN ln -s /usr/bin/clang-14 /usr/bin/clang

COPY . /go/src/github.com/microsoft/retina 
WORKDIR /go/src/github.com/microsoft/retina

# RUN go mod edit -module retina
# RUN go generate ./...

# Default linux/architecture.
ARG GOOS=linux
ENV GOOS=${GOOS}

ARG GOARCH=amd64
ENV GOARCH=${GOARCH}

ENV CGO_ENABLED=0

ARG VERSION
ARG APP_INSIGHTS_ID

RUN go build -v -o /go/bin/retina/controller -ldflags "-X main.version="$VERSION" -X "main.applicationInsightsID"="$APP_INSIGHTS_ID"" controller/main.go 
RUN go build -v -o /go/bin/retina/captureworkload -ldflags "-X main.version="$VERSION" -X "main.applicationInsightsID"="$APP_INSIGHTS_ID"" captureworkload/main.go
RUN go build -v -o /go/bin/retina/initretina -ldflags "-X main.version="$VERSION" -X "main.applicationInsightsID"="$APP_INSIGHTS_ID"" ././init/retina/main.go
