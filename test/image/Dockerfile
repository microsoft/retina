# build stage
FROM mcr.microsoft.com/oss/go/microsoft/golang:1.22 AS builder
ENV CGO_ENABLED=0

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

RUN go generate /go/src/github.com/microsoft/retina/pkg/plugin/...

# RUN go mod edit -module retina
# RUN make all generate
#RUN go generate ./...
RUN make test
#RUN go test -covermode count -coverprofile /home/runner/work/_temp/go.cov -coverpkg ./... ./...

RUN cat coverage.out

FROM scratch AS artifacts
COPY --from=builder /go/src/github.com/microsoft/retina/coverage.out /coverage.out
