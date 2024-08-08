FROM mcr.microsoft.com/oss/go/microsoft/golang:1.22 AS builder

WORKDIR /build
ADD . .
RUN go mod download 

RUN CGO_ENABLED=0 GOOS=linux go build -o kapinger .

FROM scratch
WORKDIR /build
COPY --from=builder /build/kapinger .
CMD ["./kapinger"]
