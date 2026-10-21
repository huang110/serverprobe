FROM golang:1.22-alpine AS builder

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /agent ./cmd

FROM alpine:3.19

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

COPY --from=builder /agent /app/agent
COPY config.docker.yaml /app/config.yaml

# 预留自定义脚本目录
VOLUME ["/app/scripts"]

ENTRYPOINT ["/app/agent", "-config", "/app/config.yaml"]
