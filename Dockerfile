FROM golang:1.24-alpine AS builder

WORKDIR /app
COPY . .
ARG VERSION=dev
RUN CGO_ENABLED=0 go build -ldflags "-s -w -X dash-dash-dash/internal/dash-dash-dash.buildVersion=${VERSION}" -o dash-dash-dash .

FROM alpine:3.21

# LABEL org.opencontainers.image.source="https://github.com/ShrekBytes/dash-dash-dash"

WORKDIR /app
COPY --from=builder /app/dash-dash-dash .

EXPOSE 8080/tcp
ENTRYPOINT ["/app/dash-dash-dash", "--config", "/app/config/config.yml"]
