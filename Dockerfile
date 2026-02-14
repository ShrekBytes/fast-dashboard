FROM golang:1.24-alpine AS builder

WORKDIR /app
COPY . .
RUN CGO_ENABLED=0 go build -o fast-dashboard .

FROM alpine:3.21

WORKDIR /app
COPY --from=builder /app/fast-dashboard .

EXPOSE 8080/tcp
ENTRYPOINT ["/app/fast-dashboard", "--config", "/app/config/config.yml"]
