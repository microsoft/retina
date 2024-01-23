FROM mcr.microsoft.com/oss/go/microsoft/golang:1.21 AS builder

ARG VERSION
ARG APP_INSIGHTS_ID

WORKDIR /workspace
COPY . .

RUN make manifests
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "-X main.version="$VERSION" -X "main.applicationInsightsID"="$APP_INSIGHTS_ID"" -a -o retina-operator operator/main.go

FROM scratch
WORKDIR /
COPY --from=builder /workspace/retina-operator .
USER 65532:65532

ENTRYPOINT ["/retina-operator"]
