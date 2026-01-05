FROM golang:1.21-alpine

# Install protoc, git, and wget for health checks
RUN apk add --no-cache protobuf git wget

WORKDIR /app

COPY go.mod go.sum ./
COPY consumer/ ./consumer/
COPY proto/ ./proto/

# Disable checksum verification for stability in restricted environments
ENV GOSUMDB=off

# Ensure dependencies are tidy and present
RUN go mod tidy

# Install Go protoc plugins
RUN go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.31.0
RUN go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.3.0

# Regenerate proto files
WORKDIR /app/proto
RUN protoc --go_out=. --go_opt=paths=source_relative \
    --go-grpc_out=. --go-grpc_opt=paths=source_relative \
    liquidity.proto

WORKDIR /app/consumer
RUN go build -o consumer .

CMD ["sh", "-c", "./consumer -broker kafka:29092 -liquidity liquidity-service:50051 -ws :8080 -config /config/network.json"]