FROM golang:1.22-alpine AS builder

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /serverprobe ./cmd/server

FROM alpine:3.19

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

COPY --from=builder /serverprobe /app/serverprobe
COPY config.example.yaml /app/config.yaml

EXPOSE 8080 9090

ENTRYPOINT ["/app/serverprobe", "-config", "/app/config.yaml"]
