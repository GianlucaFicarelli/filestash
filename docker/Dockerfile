FROM golang:1.26-trixie AS builder

WORKDIR /src

# Install build dependencies (dev headers only here)
RUN apt-get update && \
    apt-get install -y --no-install-recommends \
        pkg-config \
        libjpeg-dev \
        libtiff-dev \
        libpng-dev \
        libwebp-dev \
        libraw-dev \
        libheif-dev \
        libgif-dev \
        libvips-dev \
        ffmpeg \
    && rm -rf /var/lib/apt/lists/*

# Enable CGO explicitly
ENV CGO_ENABLED=1

# Cache Go modules
COPY go.mod go.sum ./
RUN go mod download

# Copy directories
COPY cmd/ ./cmd/
COPY config/ ./config/
COPY public/ ./public/
COPY server/ ./server/

# Copy individual files
COPY embed.go ./
COPY go.mod ./
COPY go.sum ./

# Inject version metadata
ARG VERSION=dev
ARG COMMIT=none
ARG DATE=unknown

RUN go build \
    -trimpath \
    -ldflags "-s -w \
        -X main.version=${VERSION} \
        -X main.commit=${COMMIT} \
        -X main.date=${DATE}" \
    --tags "fts5" \
    -o dist/filestash \
    ./cmd/main.go

FROM debian:trixie-slim

# Install runtime libraries only (NO -dev)
RUN apt-get update && \
    apt-get install -y --no-install-recommends \
        libjpeg62-turbo \
        libtiff6 \
        libpng16-16 \
        libwebp7 \
        libraw23 \
        libheif1 \
        libgif7 \
        libvips \
        ffmpeg \
        ca-certificates \
    && rm -rf /var/lib/apt/lists/*

RUN useradd -m -u 10001 filestash

USER filestash
WORKDIR /app

COPY --chown=filestash:filestash --from=builder /src/dist ./
COPY --chown=filestash:filestash ./config/config.json ./data/state/config/config.json

EXPOSE 8334

CMD ["/app/filestash"]
