# Linux builder - runs natively on the target platform (amd64 or arm64)
# skopeo inspect docker://mcr.microsoft.com/oss/go/microsoft/golang:latest --format "{{.Name}}@{{.Digest}}"
FROM mcr.microsoft.com/oss/go/microsoft/golang@sha256:aa201ad216d45d4e8c8f2512392cc8259c5daa4fa6348973235d715aa2c84c9c AS builder

WORKDIR /build
ADD . .
RUN go mod download
RUN go build -o kapinger . && mkdir -p /lib64

FROM scratch AS linux
COPY --from=builder /lib/ /lib
COPY --from=builder /lib64/ /lib64
COPY --from=builder /usr/lib/ /usr/lib
WORKDIR /app
COPY --from=builder /build/kapinger .
CMD ["./kapinger"]

# Windows builder - cross-compiles from Linux amd64 (GOOS=windows is not affected by systemcrypto)
# skopeo inspect docker://mcr.microsoft.com/oss/go/microsoft/golang:latest --format "{{.Name}}@{{.Digest}}"
FROM --platform=$BUILDPLATFORM mcr.microsoft.com/oss/go/microsoft/golang@sha256:aa201ad216d45d4e8c8f2512392cc8259c5daa4fa6348973235d715aa2c84c9c AS windows-builder

WORKDIR /build
ADD . .
RUN go mod download
RUN GOOS=windows go build -o kapinger.exe .

# skopeo inspect docker://mcr.microsoft.com/windows/servercore:ltsc2022 --override-os windows --format "{{.Name}}@{{.Digest}}"
FROM mcr.microsoft.com/windows/servercore@sha256:d4c6d1a8a1a306b12691c3b2e5e3a8bfad786cbd6b7831cd74a9a6a99eab08ad AS windows
WORKDIR /app
COPY --from=windows-builder /build/kapinger.exe .
ENTRYPOINT [ "cmd.exe" ]
CMD [ "/c", "kapinger.exe" ]
