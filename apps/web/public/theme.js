(function () {
  function storedTheme() {
    try {
      return localStorage.getItem("theme");
    } catch {
      return null;
    }
  }
  var stored = storedTheme();
  var theme = stored === "light" || stored === "dark" ? stored : "dark";
  if ((!stored || stored === "system") && matchMedia("(prefers-color-scheme: light)").matches) {
    theme = "light";
  }
  document.documentElement.classList.add(theme);
  document.documentElement.style.colorScheme = theme;
})();
