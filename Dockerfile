FROM golang:1.26-alpine AS builder

WORKDIR /workspace

COPY go.mod go.sum ./
RUN go mod download

COPY cmd/demo-agent ./cmd/demo-agent
COPY catalog ./catalog
COPY config ./config
COPY internal ./internal

RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /demo-agent ./cmd/demo-agent

FROM alpine:3.23

RUN apk add --no-cache ca-certificates \
    && addgroup -S demoagent \
    && adduser -S -G demoagent demoagent

WORKDIR /app
COPY --from=builder /demo-agent /usr/local/bin/demo-agent
COPY --from=builder /workspace/config ./config

USER demoagent
EXPOSE 8090
ENTRYPOINT ["demo-agent"]
