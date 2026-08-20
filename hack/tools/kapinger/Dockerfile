# Linux builder - runs natively on the target platform (amd64 or arm64)
# skopeo inspect docker://mcr.microsoft.com/oss/go/microsoft/golang:1.26.3 --format "{{.Name}}@{{.Digest}}"
FROM mcr.microsoft.com/oss/go/microsoft/golang:1.26.5-2@sha256:ea712a1aaf80306c19ff842ba0bfcb9ad360afd8143e70044e0d0bd6d6899887 AS builder

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
FROM --platform=$BUILDPLATFORM mcr.microsoft.com/oss/go/microsoft/golang:1.26.5-2@sha256:ea712a1aaf80306c19ff842ba0bfcb9ad360afd8143e70044e0d0bd6d6899887 AS windows-builder

WORKDIR /build
ADD . .
RUN go mod download
RUN GOOS=windows go build -o kapinger.exe .

# skopeo inspect docker://mcr.microsoft.com/windows/servercore:ltsc2022 --override-os windows --format "{{.Name}}@{{.Digest}}"
FROM mcr.microsoft.com/windows/servercore:ltsc2022@sha256:0df5b1c3394198525f426e379b61d8f09ce60cb449b1cb4cab163e965672cd67 AS windows
WORKDIR /app
COPY --from=windows-builder /build/kapinger.exe .
ENTRYPOINT [ "cmd.exe" ]
CMD [ "/c", "kapinger.exe" ]
