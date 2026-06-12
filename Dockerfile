FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -o /uigraph-api ./cmd/api

FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata
COPY --from=builder /uigraph-api /usr/local/bin/uigraph-api
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/uigraph-api"]
