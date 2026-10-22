import { createContext } from "react";

/** Each icon's Material Symbols Rounded paths: outline, then filled when it differs. */
export type TSymbolPaths = Readonly<Record<string, readonly [string, string?]>>;

let loaded: TSymbolPaths | null = null;

export async function loadSymbols() {
  if (loaded) return;
  loaded = (await import("./symbols.gen.ts")).symbolPaths;
}

export function loadedSymbols() {
  return loaded;
}

/** The symbols icons draw instead of their lucide glyph; null keeps lucide. */
export const SymbolSetContext = createContext<TSymbolPaths | null>(null);
