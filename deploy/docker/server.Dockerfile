# syntax=docker/dockerfile:1
# The Go side of Tepati: api, worker, tepatictl and goose in one image. Compose picks the
# command (`api`, `worker`, `goose ... up`). Build from the repo root: `bun run deploy:build`.

FROM golang:1.26-bookworm AS build
ARG VERSION=dev
WORKDIR /src
ENV CGO_ENABLED=0 GOFLAGS=-trimpath

# Module downloads first, so a source change does not refetch them.
COPY apps/server/go.mod apps/server/go.sum apps/server/
COPY apps/server/tools/go.mod apps/server/tools/go.sum apps/server/tools/
RUN --mount=type=cache,target=/go/pkg/mod cd apps/server && go mod download \
 && cd tools && go mod download

COPY apps/server apps/server
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build \
    cd apps/server \
 && go build -ldflags "-s -w -X github.com/andi-frame/lockedin/apps/server/internal/buildinfo.Version=${VERSION}" \
      -o /out/ ./cmd/api ./cmd/worker ./cmd/tepatictl \
 && cd tools \
 && go build -tags 'no_clickhouse no_mssql no_mysql no_sqlite3 no_vertica no_ydb no_libsql' \
      -ldflags "-s -w" -o /out/goose github.com/pressly/goose/v3/cmd/goose

FROM debian:bookworm-slim
# ffmpeg and vips are the media pipeline's tools (internal/media shells out to them).
RUN apt-get update \
 && apt-get install -y --no-install-recommends ffmpeg libvips-tools ca-certificates tzdata \
 && rm -rf /var/lib/apt/lists/* \
 && groupadd --gid 10001 tepati \
 && useradd --uid 10001 --gid 10001 --no-create-home --home-dir /nonexistent --shell /usr/sbin/nologin tepati

COPY --from=build /out/ /usr/local/bin/
COPY apps/server/db/migrations /app/migrations

USER 10001:10001
WORKDIR /app
EXPOSE 8080
CMD ["api"]
