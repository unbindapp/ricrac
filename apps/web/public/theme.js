(function () {
  function stored(key) {
    try {
      return localStorage.getItem(key);
    } catch {
      return null;
    }
  }
  var storedTheme = stored("theme");
  var theme = storedTheme === "light" || storedTheme === "dark" ? storedTheme : "dark";
  if ((!storedTheme || storedTheme === "system") && matchMedia("(prefers-color-scheme: light)").matches) {
    theme = "light";
  }
  document.documentElement.classList.add(theme);
  document.documentElement.style.colorScheme = theme;
  var storedHue = stored("ricrac:accent-hue");
  var hue = Number(storedHue);
  if (storedHue && isFinite(hue)) {
    document.documentElement.style.setProperty("--accent-hue", String(((Math.round(hue) % 360) + 360) % 360));
  }
})();
