# ssp/Dockerfile
FROM golang:1.23-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o /ssp

FROM alpine:3.17
COPY --from=builder /ssp /usr/local/bin/ssp
COPY ../config.yaml /app/config.yaml
WORKDIR /app
CMD ["/usr/local/bin/ssp"]
