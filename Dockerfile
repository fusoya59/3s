# Build stage
FROM golang:1.26-alpine AS builder

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /usr/local/bin/3s .

# Runtime stage
FROM alpine:3.21

RUN apk add --no-cache ca-certificates

RUN adduser -D -h /home/3s 3s

COPY --from=builder /usr/local/bin/3s /usr/local/bin/3s

USER 3s

ENTRYPOINT ["/usr/local/bin/3s"]
CMD ["--help"]
