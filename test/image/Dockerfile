# build stage
# skopeo inspect docker://mcr.microsoft.com/oss/go/microsoft/golang:1.24.4-azurelinux3.0 --format "{{.Name}}@{{.Digest}}"
FROM mcr.microsoft.com/oss/go/microsoft/golang@sha256:250d01e55a37bd79d7014ae83f9f50aa6fa5570ca910e7f19faeff4bb0132ae1 AS builder
ENV CGO_ENABLED=0
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
