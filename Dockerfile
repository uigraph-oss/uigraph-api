FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -o /uigraph-api ./cmd/api

# go-rod downloads a glibc Chrome at runtime, so the runtime stage is Debian-based
# (musl/Alpine can't run the downloaded binary) and ships only Chrome's shared-lib
# dependencies — not Chrome itself.
FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    fonts-liberation \
    libasound2 libatk-bridge2.0-0 libatk1.0-0 libatspi2.0-0 libcairo2 \
    libcups2 libdbus-1-3 libdrm2 libexpat1 libgbm1 libglib2.0-0 libnspr4 \
    libnss3 libpango-1.0-0 libx11-6 libxcb1 libxcomposite1 libxdamage1 \
    libxext6 libxfixes3 libxkbcommon0 libxrandr2 \
    && rm -rf /var/lib/apt/lists/*
COPY --from=builder /uigraph-api /usr/local/bin/uigraph-api
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/uigraph-api"]
