export const availableThemes = ["dark", "light", "system"] as const;
export const themes = [...availableThemes];
export type TTheme = (typeof availableThemes)[number];
export type TThemeWithoutSystem = Exclude<TTheme, "system">;
export const defaultTheme: TTheme = "system";

const themeCycle: TTheme[] = ["system", "light", "dark"];

export function nextTheme(theme: string | undefined): TTheme {
  const current = themeCycle.find((item) => item === theme) ?? defaultTheme;
  return themeCycle[(themeCycle.indexOf(current) + 1) % themeCycle.length];
}

export const DEFAULT_ACCENT_HUE = 120;
export const ACCENT_HUE_KEY = "ricrac:accent-hue";

export function accentHueOf(stored: string | null | undefined): number {
  const hue = Number(stored);
  if (stored == null || stored.trim() === "" || !Number.isFinite(hue)) return DEFAULT_ACCENT_HUE;
  return ((Math.round(hue) % 360) + 360) % 360;
}
