# syntax=docker/dockerfile:1
# Development image for the Go services: the source is bind-mounted and air rebuilds on change
# (compose.dev.yaml). Not for production; that image is server.Dockerfile.
FROM golang:1.26-bookworm
RUN apt-get update \
 && apt-get install -y --no-install-recommends ffmpeg libvips-tools \
 && rm -rf /var/lib/apt/lists/*

ENV GOFLAGS=-buildvcs=false
RUN --mount=type=cache,target=/go/pkg/mod go install github.com/air-verse/air@v1.63.0

# goose comes from the tools module, the same version `bun run db:migrate` uses.
COPY apps/server/tools/go.mod apps/server/tools/go.sum /tools/
RUN --mount=type=cache,target=/go/pkg/mod cd /tools \
 && go build -tags 'no_clickhouse no_mssql no_mysql no_sqlite3 no_vertica no_ydb no_libsql' \
      -o /usr/local/bin/goose github.com/pressly/goose/v3/cmd/goose

WORKDIR /src/apps/server
