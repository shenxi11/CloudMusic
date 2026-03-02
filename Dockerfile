FROM golang:1.22-bookworm AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN mkdir -p /out \
    && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o /out/music_server cmd/monolith/main.go \
    && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o /out/auth_server cmd/auth/main.go \
    && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o /out/catalog_server cmd/catalog/main.go \
    && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o /out/profile_server cmd/profile/main.go \
    && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o /out/media_server cmd/media/main.go \
    && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o /out/video_server cmd/video/main.go \
    && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o /out/event_worker cmd/eventworker/main.go \
    && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o /out/migrator cmd/migrator/main.go \
    && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o /out/media_indexer cmd/mediaindexer/main.go

FROM debian:bookworm-slim AS runtime

WORKDIR /app

RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates bash netcat-openbsd tzdata ffmpeg \
    && rm -rf /var/lib/apt/lists/*

COPY --from=builder /out/ /app/
COPY scripts/docker/wait_for.sh /app/scripts/docker/wait_for.sh
COPY configs/config.docker.yaml /app/configs/config.yaml
COPY migrations/sql /app/migrations/sql

RUN chmod +x /app/music_server /app/auth_server /app/catalog_server /app/profile_server /app/media_server /app/video_server /app/event_worker /app/migrator /app/media_indexer /app/scripts/docker/wait_for.sh \
    && mkdir -p /data/uploads /data/video /data/uploads_hls /app/migrations/sql

CMD ["/app/music_server"]
