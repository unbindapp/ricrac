# ---------- web ----------
FROM oven/bun:1 AS web
WORKDIR /app/apps/web
COPY apps/web/package.json apps/web/bun.lock ./
RUN bun install --frozen-lockfile
COPY apps/web ./
RUN bun x vite build

# ---------- server (embeds the web build) ----------
FROM golang:1.27-alpine AS server
ENV CGO_ENABLED=0 GOFLAGS="-trimpath -buildvcs=false"
WORKDIR /app/apps/server
COPY apps/server/go.mod apps/server/go.sum ./
RUN go mod download
COPY apps/server ./
COPY --from=web /app/apps/web/dist ./internal/web/dist
RUN --mount=type=cache,target=/root/.cache/go-build go build -ldflags "-s -w" -o /out/ricrac ./cmd/server

# ---------- claude (the assistant's CLI) ----------
FROM alpine:3.22 AS claude
ARG CLAUDE_CODE_VERSION=2.1.314
ARG TARGETARCH
RUN apk add --no-cache curl \
 && arch=$([ "$TARGETARCH" = arm64 ] && echo arm64 || echo x64) \
 && curl -fsSL "https://registry.npmjs.org/@anthropic-ai/claude-code-linux-$arch/-/claude-code-linux-$arch-$CLAUDE_CODE_VERSION.tgz" | tar xz -C /tmp \
 && install -D -m 755 /tmp/package/claude /out/claude

# ---------- runtime ----------
FROM gcr.io/distroless/cc-debian12:nonroot
COPY --from=server /out/ricrac /ricrac
COPY --from=claude /out/claude /usr/local/bin/claude
ENV PORT=3000
EXPOSE 3000
CMD ["/ricrac"]
