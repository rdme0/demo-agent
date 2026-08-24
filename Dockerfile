FROM golang:1.26-alpine AS builder

WORKDIR /workspace
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /demo-agent ./cmd/demo-agent

FROM alpine:3.23

RUN apk add --no-cache ca-certificates
COPY --from=builder /demo-agent /usr/local/bin/demo-agent
USER 65532:65532
ENTRYPOINT ["demo-agent"]
