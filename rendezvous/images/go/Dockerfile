FROM golang:1.24-bookworm AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o /app/node ./cmd/node

FROM debian:bookworm-slim
WORKDIR /app
COPY --from=builder /app/node /app/node
ENTRYPOINT ["/app/node"]
