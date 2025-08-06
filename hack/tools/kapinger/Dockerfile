FROM --platform=linux/amd64 mcr.microsoft.com/oss/go/microsoft/golang:1.24.5 AS builder

WORKDIR /build
ADD . .
RUN go mod download 

# Build for Linux
RUN CGO_ENABLED=0 GOOS=linux go build -o kapinger .

# Build for Windows
RUN CGO_ENABLED=0 GOOS=windows go build -o kapinger.exe .

# Build for ARM64 Linux
RUN CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o kapinger-arm64 .

FROM --platform=linux/amd64 scratch AS linux-amd64
WORKDIR /app
COPY --from=builder /build/kapinger .
CMD ["./kapinger"]

FROM --platform=windows/amd64 mcr.microsoft.com/windows/servercore:ltsc2022 AS windows-amd64
WORKDIR /app
COPY --from=builder /build/kapinger.exe .
ENTRYPOINT [ "cmd.exe" ]
CMD [ "/c", "kapinger.exe" ]

FROM --platform=linux/arm64 scratch AS linux-arm64
WORKDIR /app
COPY --from=builder /build/kapinger-arm64 .
CMD ["./kapinger-arm64"]
