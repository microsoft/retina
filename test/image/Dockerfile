# build stage
# skopeo inspect docker://mcr.microsoft.com/oss/go/microsoft/golang:1.26-azurelinux3.0 --format "{{.Name}}@{{.Digest}}"
FROM mcr.microsoft.com/oss/go/microsoft/golang:1.26-azurelinux3.0@sha256:829285f9358799b5b9bfb48ee11e604d09b6d7fb7bf5acc22a86f77149fd82e6 AS builder
ENV CGO_ENABLED=1
COPY . /go/src/github.com/microsoft/retina 
WORKDIR /go/src/github.com/microsoft/retina
RUN tdnf install -y clang lld bpftool libbpf-devel make git jq
RUN go generate /go/src/github.com/microsoft/retina/pkg/plugin/...
# RUN go mod edit -module retina
# RUN make all generate
#RUN go generate ./...
RUN make test
#RUN go test -covermode count -coverprofile /home/runner/work/_temp/go.cov -coverpkg ./... ./...
RUN cat coverage.out

FROM scratch AS artifacts
COPY --from=builder /go/src/github.com/microsoft/retina/coverage.out /coverage.out
