# build stage
# skopeo inspect docker://mcr.microsoft.com/oss/go/microsoft/golang:1.25.7-azurelinux3.0 --format "{{.Name}}@{{.Digest}}"
FROM mcr.microsoft.com/oss/go/microsoft/golang@sha256:408661cbcfcbf24c06fc4f85c23566b42af722fdef5a5044782859e682916be7 AS builder
COPY . /go/src/github.com/microsoft/retina
WORKDIR /go/src/github.com/microsoft/retina
RUN tdnf install -y clang lld bpftool libbpf-devel make git jq
RUN CGO_ENABLED=1 go generate -run bpf2go /go/src/github.com/microsoft/retina/pkg/plugin/...
ENV CGO_ENABLED=0
# RUN go mod edit -module retina
# RUN make all generate
#RUN go generate ./...
RUN make test
#RUN go test -covermode count -coverprofile /home/runner/work/_temp/go.cov -coverpkg ./... ./...
RUN cat coverage.out

FROM scratch AS artifacts
COPY --from=builder /go/src/github.com/microsoft/retina/coverage.out /coverage.out
