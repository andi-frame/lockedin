# syntax=docker/dockerfile:1
# The Next.js app, built as a standalone server and run by Bun. Build from the repo root:
# `bun run deploy:build`. API_INTERNAL_URL is read at run time (src/lib/api/server.ts), so the
# same image serves staging and production.

FROM oven/bun:1.4 AS build
WORKDIR /app
ENV NEXT_TELEMETRY_DISABLED=1

# Dependencies first: only the manifests, so a source change keeps this layer.
COPY package.json bun.lock ./
COPY apps/web/package.json apps/web/
RUN --mount=type=cache,target=/root/.bun/install/cache bun install --frozen-lockfile

COPY apps/web apps/web
RUN cd apps/web && bun --bun next build

FROM oven/bun:1.4-slim
WORKDIR /app
ENV NODE_ENV=production NEXT_TELEMETRY_DISABLED=1 PORT=3000 HOSTNAME=0.0.0.0
RUN groupadd --gid 10001 tepati \
 && useradd --uid 10001 --gid 10001 --no-create-home --home-dir /nonexistent --shell /usr/sbin/nologin tepati

# The standalone output keeps the monorepo layout, so server.js sits in apps/web.
COPY --from=build --chown=10001:10001 /app/apps/web/.next/standalone ./
COPY --from=build --chown=10001:10001 /app/apps/web/.next/static ./apps/web/.next/static

USER 10001:10001
EXPOSE 3000
CMD ["bun", "apps/web/server.js"]
