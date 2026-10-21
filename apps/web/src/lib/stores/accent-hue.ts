import { create } from "zustand";

import { ACCENT_HUE_KEY, accentHueOf } from "@/components/providers/themes";

function storedHue() {
  try {
    return accentHueOf(localStorage.getItem(ACCENT_HUE_KEY));
  } catch {
    return accentHueOf(null);
  }
}

function applyHue(hue: number) {
  document.documentElement.style.setProperty("--accent-hue", String(hue));
  try {
    localStorage.setItem(ACCENT_HUE_KEY, String(hue));
  } catch {
    return;
  }
}

type TAccentHueStore = {
  hue: number;
  setHue: (hue: number) => void;
};

/** The hue the theme's accent colors are built from, saved in this browser. */
export const useAccentHueStore = create<TAccentHueStore>((set) => ({
  hue: storedHue(),
  setHue: (hue) => {
    const next = accentHueOf(String(hue));
    applyHue(next);
    set({ hue: next });
  },
}));
