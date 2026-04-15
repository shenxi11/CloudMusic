FROM golang:1.22-bookworm AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY cmd ./cmd
COPY internal ./internal
COPY pkg ./pkg

RUN set -eux; \
    mkdir -p /out; \
    for item in \
      "music_server:cmd/monolith/main.go" \
      "auth_server:cmd/auth/main.go" \
      "catalog_server:cmd/catalog/main.go" \
      "profile_server:cmd/profile/main.go" \
      "media_server:cmd/media/main.go" \
      "video_server:cmd/video/main.go" \
      "event_worker:cmd/eventworker/main.go" \
      "migrator:cmd/migrator/main.go" \
      "media_indexer:cmd/mediaindexer/main.go"; \
    do \
      name="${item%%:*}"; \
      main="${item#*:}"; \
      CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o "/out/${name}" "${main}"; \
    done

FROM debian:bookworm-slim AS runtime

WORKDIR /app

RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates bash netcat-openbsd tzdata ffmpeg

COPY --from=builder /out/ /app/
COPY scripts/docker/wait_for.sh /app/scripts/docker/wait_for.sh
COPY configs/config.docker.yaml /app/configs/config.yaml
COPY migrations/sql /app/migrations/sql

RUN chmod +x /app/music_server /app/auth_server /app/catalog_server /app/profile_server /app/media_server /app/video_server /app/event_worker /app/migrator /app/media_indexer /app/scripts/docker/wait_for.sh \
    && mkdir -p /data/uploads /data/video /data/uploads_hls /app/migrations/sql

CMD ["/app/music_server"]
