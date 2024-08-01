# build stage
# mcr.microsoft.com/oss/go/microsoft/golang:1.22.5-1-cbl-mariner2.0
FROM mcr.microsoft.com/oss/go/microsoft/golang@sha256:d3aa879f8a636fc0c7ac0108bb9f0738a7782e02a0e183dda80a974cd110efbd as builder
ENV CGO_ENABLED=0
COPY . /go/src/github.com/microsoft/retina 
WORKDIR /go/src/github.com/microsoft/retina
RUN tdnf install -y clang16 lld16 bpftool libbpf-devel make git
RUN go generate /go/src/github.com/microsoft/retina/pkg/plugin/...
# RUN go mod edit -module retina
# RUN make all generate
#RUN go generate ./...
RUN make test
#RUN go test -covermode count -coverprofile /home/runner/work/_temp/go.cov -coverpkg ./... ./...
RUN cat coverage.out

FROM scratch AS artifacts
COPY --from=builder /go/src/github.com/microsoft/retina/coverage.out /coverage.out
