# Build stage
FROM golang:1.21-bullseye AS builder

WORKDIR /go/src/github.com/microsoft/retina
COPY . .

# Build the e2e test binary
RUN go test -c -o /go/bin/e2e.test -tags=e2e ./test/e2e/

# Runtime stage
FROM debian:bullseye-slim

# Install required runtime dependencies
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    && apt-get clean \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

# Copy the built test binary from the builder stage
COPY --from=builder /go/bin/e2e.test /app/
COPY test/e2e/entrypoint.sh /app/

# Make the entrypoint script executable
RUN chmod +x /app/entrypoint.sh

ENTRYPOINT ["/app/entrypoint.sh"]