# skopeo inspect docker://mcr.microsoft.com/oss/go/microsoft/golang:1.26.2-azurelinux3.0 --format "{{.Name}}@{{.Digest}}"
FROM --platform=$BUILDPLATFORM mcr.microsoft.com/oss/go/microsoft/golang:1.26.2-azurelinux3.0@sha256:d904c53bb6a3a572482ff630fbd90cf487d5b8b3c4c8fe0e1d0b59d5d251bc8c AS builder

ARG VERSION
ARG APP_INSIGHTS_ID

WORKDIR /workspace
COPY . .

RUN tdnf install -y jq

# Default linux/architecture.
ARG GOOS=linux
ENV GOOS=${GOOS}

ARG GOARCH=amd64
ENV GOARCH=${GOARCH}

RUN make manifests
RUN --mount=type=cache,target="/root/.cache/go-build" \
	GOOS=$GOOS GOARCH=$GOARCH go build \
	-ldflags "-X github.com/microsoft/retina/internal/buildinfo.Version="$VERSION" \
	-X "github.com/microsoft/retina/internal/buildinfo.ApplicationInsightsID"="$APP_INSIGHTS_ID"" \
	-a -o retina-operator operator/main.go

##################### controller #######################
# skopeo inspect docker://mcr.microsoft.com/azurelinux/distroless/minimal:3.0 --format "{{.Name}}@{{.Digest}}"
FROM --platform=$BUILDPLATFORM mcr.microsoft.com/azurelinux/distroless/minimal:3.0@sha256:5a66f9f16ac675db2a8229dac72d83811b73b502d6ad192d8b374c7f3be498af
WORKDIR /
COPY --from=builder /lib /lib
COPY --from=builder /usr/lib/ /usr/lib
COPY --from=builder /etc/pki/tls/ /etc/pki/tls/
COPY --from=builder /workspace/retina-operator .
USER 65532:65532

ENTRYPOINT ["/retina-operator"]
