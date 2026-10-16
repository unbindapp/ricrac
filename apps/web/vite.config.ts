import path from "node:path"
import tailwindcss from "@tailwindcss/vite"
import { tanstackRouter } from "@tanstack/router-plugin/vite"
import react from "@vitejs/plugin-react"
import { defineConfig } from "vite"

// In development Vite serves the SPA and hands everything the Rust server owns to it, so
// PUBLIC_URL can be http://localhost:5173 and cookies, redirects and the SPA share one origin.
// /oauth/authorize is the SPA's consent page, so it isn't proxied.
const server = "http://localhost:3000"
const serverPaths = [
  "/api",
  "/v1",
  "/mcp",
  "/oauth/register",
  "/oauth/token",
  "/oauth/revoke",
  "/.well-known",
  "/files",
  "/uploads",
  "/llms.txt",
  "/healthz",
]

export default defineConfig({
  plugins: [tanstackRouter({ target: "react", autoCodeSplitting: true }), react(), tailwindcss()],
  resolve: { alias: { "@": path.resolve(import.meta.dirname, "./src") } },
  server: { proxy: Object.fromEntries(serverPaths.map((p) => [p, { target: server }])) },
})
