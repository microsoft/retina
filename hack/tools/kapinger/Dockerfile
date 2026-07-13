# Linux builder - runs natively on the target platform (amd64 or arm64)
# skopeo inspect docker://mcr.microsoft.com/oss/go/microsoft/golang:1.26.3 --format "{{.Name}}@{{.Digest}}"
FROM mcr.microsoft.com/oss/go/microsoft/golang:1.26.4-1@sha256:2f7f3e209b4b129588a6dd1b1579aa4d79e20e2671ae9ba3482cd37a337c8061 AS builder

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
# skopeo inspect docker://mcr.microsoft.com/oss/go/microsoft/golang:1.26.3 --format "{{.Name}}@{{.Digest}}"
FROM --platform=$BUILDPLATFORM mcr.microsoft.com/oss/go/microsoft/golang:1.26.4-1@sha256:2f7f3e209b4b129588a6dd1b1579aa4d79e20e2671ae9ba3482cd37a337c8061 AS windows-builder

WORKDIR /build
ADD . .
RUN go mod download
RUN GOOS=windows go build -o kapinger.exe .

# skopeo inspect docker://mcr.microsoft.com/windows/servercore:ltsc2022 --override-os windows --format "{{.Name}}@{{.Digest}}"
FROM mcr.microsoft.com/windows/servercore:ltsc2022@sha256:a23b350061d76236e2c427e32175a2decfe3214200eee4ae9ee9cd9e98f26bf0 AS windows
WORKDIR /app
COPY --from=windows-builder /build/kapinger.exe .
ENTRYPOINT [ "cmd.exe" ]
CMD [ "/c", "kapinger.exe" ]
