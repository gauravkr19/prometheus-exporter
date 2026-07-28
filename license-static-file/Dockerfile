# Start with the official Golang image as the build environment
FROM golang:1.22-alpine AS builder
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o /app/license_exporter

FROM alpine:3.18
WORKDIR /
COPY --from=builder /app/license_exporter /license_exporter
COPY license_info.yaml /license_info.yaml

CMD ["/license_exporter"]
