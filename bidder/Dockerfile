# bidder/Dockerfile
FROM golang:1.20-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o /bidder

FROM alpine:3.17
COPY --from=builder /bidder /usr/local/bin/bidder
COPY ../config.yaml /app/config.yaml
WORKDIR /app
CMD ["/usr/local/bin/bidder"]
