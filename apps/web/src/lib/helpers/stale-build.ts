const RELOAD_KEY = "ricrac:stale-build-reload";
const RELOAD_COOLDOWN_MS = 10 * 1000;

/** Whether to reload for a newer build: not again within a few seconds, so a broken build can't loop. */
export function shouldReloadForBuild(lastReloadAt: number | null, now: number) {
  return lastReloadAt === null || now - lastReloadAt > RELOAD_COOLDOWN_MS;
}

/** Reloads once when a page made before a deploy asks for code the new build no longer has. */
export function reloadOnStaleBuild() {
  window.addEventListener("vite:preloadError", (event) => {
    const stored = Number(sessionStorage.getItem(RELOAD_KEY));
    const now = Date.now();
    if (!shouldReloadForBuild(stored || null, now)) return;
    event.preventDefault();
    sessionStorage.setItem(RELOAD_KEY, String(now));
    window.location.reload();
  });
}
