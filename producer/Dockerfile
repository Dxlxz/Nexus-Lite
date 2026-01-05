FROM golang:1.21-alpine

# Install wget for health checks
RUN apk add --no-cache wget

WORKDIR /app

COPY go.mod go.sum ./
COPY producer/ ./producer/

# Disable checksum verification for stability
ENV GOSUMDB=off

# Ensure dependencies are tidy and present
RUN go mod tidy

WORKDIR /app/producer
RUN go build -o producer .

CMD ["sh", "-c", "./producer -broker kafka:29092 -tps 100 -config /config/network.json -health :8081"]