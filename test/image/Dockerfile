# build stage
# mcr.microsoft.com/oss/go/microsoft/golang:1.23.5-cbl-mariner2.0
FROM mcr.microsoft.com/oss/go/microsoft/golang@sha256:ef39e430a97a61bec3395adb673a7c5611d2212f14abe40da21cbfd11a9d6e9d AS builder
ENV CGO_ENABLED=0
COPY . /go/src/github.com/microsoft/retina 
WORKDIR /go/src/github.com/microsoft/retina
RUN tdnf install -y clang16 lld16 bpftool libbpf-devel make git jq
RUN go generate /go/src/github.com/microsoft/retina/pkg/plugin/...
# RUN go mod edit -module retina
# RUN make all generate
#RUN go generate ./...
RUN make test
#RUN go test -covermode count -coverprofile /home/runner/work/_temp/go.cov -coverpkg ./... ./...
RUN cat coverage.out

FROM scratch AS artifacts
COPY --from=builder /go/src/github.com/microsoft/retina/coverage.out /coverage.out
