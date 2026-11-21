import {
  argbFromHex,
  argbFromRgb,
  Blend,
  Hct,
  hexFromArgb,
  TonalPalette,
} from "@material/material-color-utilities";

type TTones = Record<number, string>;
type TKeyPalette = "primary" | "error" | "neutral" | "neutral-variant";

/** The hue of the Dux purple #5f598b, as the accent picker counts it. */
export const BASELINE_HUE = 247;

const BASELINE_SATURATION = 0.219;
const BASELINE_LIGHTNESS = 0.447;

/** The Dux scheme's tones: one purple accent over near-neutral surfaces. */
const baselinePalettes: Record<TKeyPalette, TTones> = {
  primary: {
    27: "#403c60",
    29: "#443e6d",
    35: "#534e73",
    40: "#5f598b",
    80: "#c8c1ed",
    82: "#cdc5fe",
    90: "#e6dfff",
    98: "#fcf7ff",
    100: "#ffffff",
  },
  error: {
    20: "#67001f",
    22: "#6e0523",
    30: "#871c34",
    40: "#a8364b",
    65: "#f97386",
    74: "#ff97a3",
    100: "#ffffff",
  },
  neutral: {
    0: "#000000",
    4: "#0e0e12",
    6: "#141318",
    9: "#1a191f",
    12: "#201f26",
    15: "#26252d",
    21: "#33313a",
    24: "#3a383e",
    36: "#565459",
    65: "#9f9ca2",
    87: "#ddd8e0",
    90: "#e5e0ed",
    91: "#e8e3f0",
    92: "#ebe6f1",
    94: "#f1ecf6",
    96: "#f7f2fa",
    98: "#fcf8fe",
    100: "#ffffff",
  },
  "neutral-variant": {
    15: "#26252d",
    40: "#605e68",
    49: "#77747e",
    51: "#7c7984",
    70: "#ada9b5",
    90: "#e5e0ed",
  },
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

/** Key palettes for a picked hue: the Dux purple turned to it, with its chromas. */
function generatedPalettes(accentHue: number): Record<TKeyPalette, TTones> {
  const { hue, chroma } = Hct.fromInt(
    argbFromHsl(accentHue, BASELINE_SATURATION, BASELINE_LIGHTNESS),
  );
  const palette = (name: TKeyPalette, keyHue: number, keyChroma: number) =>
    tonesOf(TonalPalette.fromHueAndChroma(keyHue, keyChroma), tonesFor(baselinePalettes[name]));
  return {
    primary: palette("primary", hue, Math.max(32, chroma)),
    error: palette("error", 12, 60),
    neutral: palette("neutral", hue, 6),
    "neutral-variant": palette("neutral-variant", hue, 8.5),
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
