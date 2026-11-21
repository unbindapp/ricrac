import {
  argbFromHex,
  argbFromRgb,
  Blend,
  Hct,
  hexFromArgb,
  TonalPalette,
} from "@material/material-color-utilities";

type TTones = Record<number, string>;
type TKeyPalette = "primary" | "secondary" | "tertiary" | "error" | "neutral" | "neutral-variant";

/** The hue of Material 3's baseline seed #6750A4, DankMaterialShell's purple, as the accent picker counts it. */
export const BASELINE_HUE = 256;

const BASELINE_SATURATION = 0.344;
const BASELINE_LIGHTNESS = 0.478;

/** The baseline scheme's key palettes, from Compose Material3's PaletteTokens. */
const baselinePalettes: Record<TKeyPalette, TTones> = {
  primary: {
    10: "#21005d",
    20: "#381e72",
    30: "#4f378b",
    40: "#6750a4",
    80: "#d0bcff",
    90: "#eaddff",
    100: "#ffffff",
  },
  secondary: {
    10: "#1d192b",
    20: "#332d41",
    30: "#4a4458",
    40: "#625b71",
    80: "#ccc2dc",
    90: "#e8def8",
    100: "#ffffff",
  },
  tertiary: {
    10: "#31111d",
    20: "#492532",
    30: "#633b48",
    40: "#7d5260",
    80: "#efb8c8",
    90: "#ffd8e4",
    100: "#ffffff",
  },
  error: {
    10: "#410e0b",
    20: "#601410",
    30: "#8c1d18",
    40: "#b3261e",
    80: "#f2b8b5",
    90: "#f9dedc",
    100: "#ffffff",
  },
  neutral: {
    0: "#000000",
    4: "#0f0d13",
    6: "#141218",
    10: "#1d1b20",
    12: "#211f26",
    17: "#2b2930",
    20: "#322f35",
    22: "#36343b",
    24: "#3b383e",
    87: "#ded8e1",
    90: "#e6e0e9",
    92: "#ece6f0",
    94: "#f3edf7",
    95: "#f5eff7",
    96: "#f7f2fa",
    98: "#fef7ff",
    100: "#ffffff",
  },
  "neutral-variant": { 30: "#49454f", 50: "#79747e", 60: "#938f99", 80: "#cac4d0", 90: "#e7e0ec" },
};

const customColors = {
  green: "#1f6b3a",
  orange: "#9a4a15",
  teal: "#146a70",
  yellow: "#6e6500",
  blue: "#005cab",
};

const customColorTones = [10, 20, 30, 40, 80, 90, 100];

function argbFromHsl(hue: number, saturation: number, lightness: number) {
  const amount = saturation * Math.min(lightness, 1 - lightness);
  const channel = (n: number) => {
    const k = (n + hue / 30) % 12;
    return Math.round((lightness - amount * Math.max(-1, Math.min(k - 3, 9 - k, 1))) * 255);
  };
  return argbFromRgb(channel(0), channel(8), channel(4));
}

function tonesOf(palette: TonalPalette, tones: number[]): TTones {
  return Object.fromEntries(tones.map((tone) => [tone, hexFromArgb(palette.tone(tone))]));
}

function tonesFor(tones: TTones) {
  return Object.keys(tones).map(Number);
}

/** Key palettes for a picked hue: the baseline purple turned to it, with the baseline's chromas. */
function generatedPalettes(accentHue: number): Record<TKeyPalette, TTones> {
  const { hue, chroma } = Hct.fromInt(
    argbFromHsl(accentHue, BASELINE_SATURATION, BASELINE_LIGHTNESS),
  );
  const palette = (name: TKeyPalette, keyHue: number, keyChroma: number) =>
    tonesOf(TonalPalette.fromHueAndChroma(keyHue, keyChroma), tonesFor(baselinePalettes[name]));
  return {
    primary: palette("primary", hue, Math.max(48, chroma)),
    secondary: palette("secondary", hue, 16),
    tertiary: palette("tertiary", hue + 60, 24),
    error: palette("error", 25, 84),
    neutral: palette("neutral", hue, 6),
    "neutral-variant": palette("neutral-variant", hue, 8),
  };
}

/** Custom colors harmonized toward the primary, as Material Theme Builder does. */
function customPalettes(primary: string): Record<string, TTones> {
  const source = argbFromHex(primary);
  return Object.fromEntries(
    Object.entries(customColors).map(([name, hex]) => {
      const color = Hct.fromInt(Blend.harmonize(argbFromHex(hex), source));
      return [
        name,
        tonesOf(TonalPalette.fromHueAndChroma(color.hue, color.chroma), customColorTones),
      ];
    }),
  );
}

/** Every tone the color roles use, as --palette-<name>-<tone> variables. */
export function paletteVariables(accentHue: number | null): Record<string, string> {
  const keyPalettes = accentHue === null ? baselinePalettes : generatedPalettes(accentHue);
  const palettes = { ...keyPalettes, ...customPalettes(keyPalettes.primary[40]) };
  return Object.fromEntries(
    Object.entries(palettes).flatMap(([name, tones]) =>
      Object.entries(tones).map(([tone, hex]) => [`--palette-${name}-${tone}`, hex]),
    ),
  );
}

export function paintPalette(accentHue: number | null) {
  const style = document.documentElement.style;
  for (const [name, value] of Object.entries(paletteVariables(accentHue))) {
    style.setProperty(name, value);
  }
}
