# build stage
# skopeo inspect docker://mcr.microsoft.com/oss/go/microsoft/golang:1.26.2-azurelinux3.0 --format "{{.Name}}@{{.Digest}}"
FROM mcr.microsoft.com/oss/go/microsoft/golang:1.26.2-azurelinux3.0@sha256:ee3b11755cb3401848dc3140bbaae8da81dce6f7c7569857b2f45f040ae8868b AS builder
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
