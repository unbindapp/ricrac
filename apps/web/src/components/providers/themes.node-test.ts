import assert from "node:assert/strict";
import { test } from "node:test";

import { accentHueOf, DEFAULT_ACCENT_HUE, nextTheme } from "./themes.ts";

test("toggling cycles system, light, dark and back", () => {
  assert.equal(nextTheme("system"), "light");
  assert.equal(nextTheme("light"), "dark");
  assert.equal(nextTheme("dark"), "system");
});

test("an unset or unknown theme toggles as if it were system", () => {
  assert.equal(nextTheme(undefined), "light");
  assert.equal(nextTheme("sepia"), "light");
});

test("a stored accent hue is read as a whole degree on the color wheel", () => {
  assert.equal(accentHueOf("140"), 140);
  assert.equal(accentHueOf("12.6"), 13);
  assert.equal(accentHueOf("400"), 40);
  assert.equal(accentHueOf("-30"), 330);
});

test("a missing or broken accent hue falls back to the default", () => {
  assert.equal(accentHueOf(null), DEFAULT_ACCENT_HUE);
  assert.equal(accentHueOf(""), DEFAULT_ACCENT_HUE);
  assert.equal(accentHueOf("blue"), DEFAULT_ACCENT_HUE);
});
