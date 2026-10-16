# Linux builder - runs natively on the target platform (amd64 or arm64)
# skopeo inspect docker://mcr.microsoft.com/oss/go/microsoft/golang:1.26 --format "{{.Name}}@{{.Digest}}"
FROM mcr.microsoft.com/oss/go/microsoft/golang:1.26@sha256:71a8dd79ac8a79bdf636e015eb2a7dec2d7ec6d81cc3624f928ced8714b621d0 AS builder

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
# skopeo inspect docker://mcr.microsoft.com/oss/go/microsoft/golang:1.26 --format "{{.Name}}@{{.Digest}}"
FROM --platform=$BUILDPLATFORM mcr.microsoft.com/oss/go/microsoft/golang:1.26@sha256:71a8dd79ac8a79bdf636e015eb2a7dec2d7ec6d81cc3624f928ced8714b621d0 AS windows-builder

WORKDIR /build
ADD . .
RUN go mod download
RUN GOOS=windows go build -o kapinger.exe .

# skopeo inspect docker://mcr.microsoft.com/windows/servercore:ltsc2022 --override-os windows --format "{{.Name}}@{{.Digest}}"
FROM mcr.microsoft.com/windows/servercore:ltsc2022@sha256:593cf4a5ef08cfff00a03637e45eba073b8fe9dc90b80af4449bc5f427c7cdcd AS windows
WORKDIR /app
COPY --from=windows-builder /build/kapinger.exe .
ENTRYPOINT [ "cmd.exe" ]
CMD [ "/c", "kapinger.exe" ]
