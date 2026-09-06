# syntax=docker/dockerfile:1
FROM golang:1.24-bookworm AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY . .
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go build -o /app/node ./cmd/node

FROM debian:bookworm-slim
WORKDIR /app
COPY --from=builder /app/node /app/node
ENTRYPOINT ["/app/node"]
