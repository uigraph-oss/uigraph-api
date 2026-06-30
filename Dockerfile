FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -o /uigraph-api ./cmd/api

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    wget \
    fonts-liberation \
    chromium \
    && rm -rf /var/lib/apt/lists/*
ENV UIGRAPH_CHROMIUM_PATH=/usr/bin/chromium
COPY --from=builder /uigraph-api /usr/local/bin/uigraph-api
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/uigraph-api"]
