# Build stage
FROM golang:1.26-alpine AS builder

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /usr/local/bin/3s .

# Runtime stage
FROM debian:stable-slim

RUN apt-get update && DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends \
	ca-certificates \
	adduser \
	fonts-liberation \
	libasound2t64 \
	libatk-bridge2.0-0t64 \
	libatk1.0-0t64 \
	libatspi2.0-0t64 \
	libcairo2 \
	libcups2t64 \
	libcurl4t64 \
	libdrm2 \
	libexpat1 \
	libgbm1 \
	libglib2.0-0t64 \
	libnspr4 \
	libnss3 \
	libpango-1.0-0 \
	libx11-6 \
	libxcb1 \
	libxcomposite1 \
	libxdamage1 \
	libxext6 \
	libxfixes3 \
	libxkbcommon0 \
	libxrandr2 \
	libudev1 \
	&& rm -rf /var/lib/apt/lists/*

RUN adduser --disabled-password --home /home/3s --allow-bad-names --gecos '' 3s

COPY --from=builder /usr/local/bin/3s /usr/local/bin/3s

ENV HOME=/home/3s

ENTRYPOINT ["/usr/local/bin/3s"]
CMD ["--help"]
