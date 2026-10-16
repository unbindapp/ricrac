# ---------- web ----------
FROM oven/bun:1 AS web
WORKDIR /app/apps/web
COPY apps/web/package.json apps/web/bun.lock ./
RUN bun install --frozen-lockfile
COPY apps/web ./
RUN bun run build

# ---------- rust toolchain ----------
# Pinned so a new Rust release doesn't throw away the cached dependency layers.
FROM rust:1.98-bookworm AS chef
RUN curl -L --proto '=https' --tlsv1.2 -sSf https://raw.githubusercontent.com/cargo-bins/cargo-binstall/main/install-from-binstall-release.sh | bash \
    && cargo binstall -y cargo-chef@0.1.78
WORKDIR /app/apps/server

FROM chef AS planner
COPY apps/server ./
RUN cargo chef prepare --recipe-path recipe.json

# ---------- server (embeds the web build) ----------
# The build step keeps only the binary: its layer changes every build.
FROM chef AS server
COPY --from=planner /app/apps/server/recipe.json recipe.json
RUN cargo chef cook --release --recipe-path recipe.json
COPY apps/server ./
COPY --from=web /app/apps/web/dist /app/apps/web/dist
RUN cargo build --release \
    && mkdir /out && mv target/release/ricrac /out/ \
    && rm -rf target

# ---------- runtime ----------
FROM debian:bookworm-slim

RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates curl \
    && rm -rf /var/lib/apt/lists/*

COPY --from=server /out/ricrac /usr/local/bin/ricrac

ENV PORT=3000 \
    RUST_LOG=info

EXPOSE 3000

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s CMD curl -fsS "http://127.0.0.1:${PORT}/healthz" || exit 1

CMD ["ricrac"]
