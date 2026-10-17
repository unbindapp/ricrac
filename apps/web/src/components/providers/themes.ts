export const availableThemes = ["dark", "light", "system"] as const;
export const themes = [...availableThemes];
export type TTheme = (typeof availableThemes)[number];
export type TThemeWithoutSystem = Exclude<TTheme, "system">;
export const defaultTheme: TTheme = "system";

export const metaTheme: Record<TThemeWithoutSystem, string> = {
  dark: "#09090A",
  light: "#F4F4F5",
};

const themeCycle: TTheme[] = ["system", "light", "dark"];

export function nextTheme(theme: string | undefined): TTheme {
  const current = themeCycle.find((item) => item === theme) ?? defaultTheme;
  return themeCycle[(themeCycle.indexOf(current) + 1) % themeCycle.length];
}
