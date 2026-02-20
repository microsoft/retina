# skopeo inspect docker://mcr.microsoft.com/oss/go/microsoft/golang:1.26.0 --format "{{.Name}}@{{.Digest}}"
FROM --platform=linux/amd64 mcr.microsoft.com/oss/go/microsoft/golang@sha256:785a5925167647ecf62512a807a32803aaef9716b90a4cf8af9b35bf31f817d2 AS builder

WORKDIR /build
ADD . .
RUN go mod download 

# Build for Linux
RUN GOOS=linux go build -o kapinger .

# Build for Windows
RUN GOOS=windows go build -o kapinger.exe .

# Build for ARM64 Linux
RUN GOOS=linux GOARCH=arm64 go build -o kapinger-arm64 .

FROM --platform=linux/amd64 scratch AS linux-amd64
COPY --from=builder /lib/ /lib
COPY --from=builder /usr/lib/ /usr/lib
WORKDIR /app
COPY --from=builder /build/kapinger .
CMD ["./kapinger"]

# skopeo inspect docker://mcr.microsoft.com/windows/servercore:ltsc2022 --override-os windows --format "{{.Name}}@{{.Digest}}"
FROM --platform=windows/amd64 mcr.microsoft.com/windows/servercore@sha256:a264df8cd8c329eed3fd1e0cafcd4f3dc453e2c72a277f9bb140fd6f10a2eefc AS windows-amd64
WORKDIR /app
COPY --from=builder /build/kapinger.exe .
ENTRYPOINT [ "cmd.exe" ]
CMD [ "/c", "kapinger.exe" ]

FROM --platform=linux/arm64 scratch AS linux-arm64
COPY --from=builder /lib/ /lib
COPY --from=builder /usr/lib/ /usr/lib
WORKDIR /app
COPY --from=builder /build/kapinger-arm64 .
CMD ["./kapinger-arm64"]
