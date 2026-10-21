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

# ---------- runtime ----------
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=server /out/ricrac /ricrac
ENV PORT=3000
EXPOSE 3000
CMD ["/ricrac"]
