# Linux builder - runs natively on the target platform (amd64 or arm64)
# skopeo inspect docker://mcr.microsoft.com/oss/go/microsoft/golang:1.26.3 --format "{{.Name}}@{{.Digest}}"
FROM mcr.microsoft.com/oss/go/microsoft/golang:1.26.3@sha256:9cb93c7fc65fcec28b0af140bd4e42510e7553dd67718289dfee2a955986b7e3 AS builder

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
FROM --platform=$BUILDPLATFORM mcr.microsoft.com/oss/go/microsoft/golang:1.26.3@sha256:9cb93c7fc65fcec28b0af140bd4e42510e7553dd67718289dfee2a955986b7e3 AS windows-builder

WORKDIR /build
ADD . .
RUN go mod download
RUN GOOS=windows go build -o kapinger.exe .

# skopeo inspect docker://mcr.microsoft.com/windows/servercore:ltsc2022 --override-os windows --format "{{.Name}}@{{.Digest}}"
FROM mcr.microsoft.com/windows/servercore:ltsc2022@sha256:86da395cfd2b35dbfc2e9d08719550c51b0570c394bff8f92622a19234766185 AS windows
WORKDIR /app
COPY --from=windows-builder /build/kapinger.exe .
ENTRYPOINT [ "cmd.exe" ]
CMD [ "/c", "kapinger.exe" ]
