// Follows the system color scheme. Loaded before the app so the first paint has the right colors.
const scheme = window.matchMedia("(prefers-color-scheme: dark)")
const apply = () => document.documentElement.classList.toggle("dark", scheme.matches)
apply()
scheme.addEventListener("change", apply)
