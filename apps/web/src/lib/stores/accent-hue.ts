import { create } from "zustand";

import { ACCENT_HUE_KEY, accentHueOf } from "@/components/providers/themes";

function storedHue() {
  try {
    const stored = localStorage.getItem(ACCENT_HUE_KEY);
    return stored ? accentHueOf(stored) : null;
  } catch {
    return null;
  }
}

let pendingFrame: number | null = null;

/** At most once a frame: every hued color on the page is recomputed from it. */
function paintHue(hue: number | null) {
  if (pendingFrame !== null) cancelAnimationFrame(pendingFrame);
  pendingFrame = requestAnimationFrame(() => {
    pendingFrame = null;
    const root = document.documentElement;
    if (hue === null) root.style.removeProperty("--accent-hue");
    else root.style.setProperty("--accent-hue", String(hue));
  });
}

function storeHue(hue: number | null) {
  try {
    if (hue === null) localStorage.removeItem(ACCENT_HUE_KEY);
    else localStorage.setItem(ACCENT_HUE_KEY, String(hue));
  } catch {
    return;
  }
}

type TAccentHueStore = {
  /** Null keeps the workspace theme's own hue. */
  hue: number | null;
  setHue: (hue: number | null) => void;
  /** Shows a hue while it is dragged, without saving it. */
  previewHue: (hue: number) => void;
};

/** The hue the theme's accent colors are built from, saved in this browser. */
export const useAccentHueStore = create<TAccentHueStore>((set) => ({
  hue: storedHue(),
  setHue: (hue) => {
    const next = hue === null ? null : accentHueOf(String(hue));
    paintHue(next);
    storeHue(next);
    set({ hue: next });
  },
  previewHue: (hue) => {
    const next = accentHueOf(String(hue));
    paintHue(next);
    set({ hue: next });
  },
}));
