FROM mcr.microsoft.com/oss/go/microsoft/golang:1.21 AS builder

WORKDIR /app
COPY ./hack/tools .
RUN go mod download

RUN CGO_ENABLED=0 GOOS=linux go build -o app ./kapinger

FROM scratch
WORKDIR /app
COPY --from=builder /app/app .
CMD ["./app"]
