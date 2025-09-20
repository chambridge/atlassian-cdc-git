# Build stage
FROM golang:1.21-alpine AS builder

WORKDIR /workspace

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY operands/api/ ./operands/api/
COPY internal/ ./internal/

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o api-server ./operands/api/main.go

# Runtime stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates git openssh-client
WORKDIR /root/

# Copy the binary from builder stage
COPY --from=builder /workspace/api-server .

EXPOSE 8080

CMD ["./api-server"]