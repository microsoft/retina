# build stage
# skopeo inspect docker://mcr.microsoft.com/oss/go/microsoft/golang:azurelinux3.0 --format "{{.Name}}@{{.Digest}}"
FROM mcr.microsoft.com/oss/go/microsoft/golang@sha256:42739e5cb9703e3e0908d0e393ef9e0acde9d7e2ef09edf7b910eda2ebde8eaf AS builder
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
