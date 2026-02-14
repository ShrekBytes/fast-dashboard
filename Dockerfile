FROM golang:1.24-alpine AS builder

WORKDIR /app
COPY . .
RUN CGO_ENABLED=0 go build -o dash-dash-dash .

FROM alpine:3.21

WORKDIR /app
COPY --from=builder /app/dash-dash-dash .

EXPOSE 8080/tcp
ENTRYPOINT ["/app/dash-dash-dash", "--config", "/app/config/config.yml"]
