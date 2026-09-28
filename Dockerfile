# Multi-stage Dockerfile for AgentMemory Universal (Pure Go, Zero-CGO)
FROM golang:1.24-alpine AS builder

WORKDIR /src

RUN apk add --no-cache git ca-certificates

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o /bin/agent-memory ./cmd/agent-memory

# Minimal runtime image
FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

COPY --from=builder /bin/agent-memory /usr/local/bin/agent-memory

ENV AGENT_MEMORY_DB=/data/memory.db
VOLUME ["/data"]

ENTRYPOINT ["agent-memory"]
CMD ["serve"]
