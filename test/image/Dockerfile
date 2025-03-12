# build stage
# skopeo inspect docker://mcr.microsoft.com/oss/go/microsoft/golang:1.24.1-2-cbl-mariner2.0 --format "{{.Name}}@{{.Digest}}"
FROM mcr.microsoft.com/oss/go/microsoft/golang@sha256:00e2221efa6ea3f01b4408432e94ad2cddf5a44fad5882adb7ec5cbcf61a0ec8 AS builder
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
