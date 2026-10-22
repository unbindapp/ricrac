import { clsx, type ClassValue } from "clsx";
import { extendTailwindMerge } from "tailwind-merge";

import { UI_THEME_IDS } from "../../themes/types.ts";

const twMerge = extendTailwindMerge({
  extend: {
    classGroups: {
      shadow: [
        {
          shadow: [
            "navbar",
            "dialog",
            "card-highlight",
            "md",
            "block-card-highlight-active",
            "block-card-highlight-idle",
          ],
        },
      ],
    },
  },
});

const themeVariants = UI_THEME_IDS.filter((id) => id !== "default").map((id) => `${id}:`);
const themed = new Map<string, string>();

function isThemed(className: string) {
  return themeVariants.some((variant) => className.startsWith(variant));
}

function unthemed(className: string) {
  return className.slice(className.indexOf(":") + 1);
}

function conflicts(earlier: string, later: string) {
  return twMerge(earlier, later) === later || twMerge(later, earlier) === earlier;
}

function isOverridden(className: string, laterPlain: string[]) {
  const plain = unthemed(className);
  return laterPlain.some((later) => conflicts(plain, later));
}

/** A theme's class gives way to a plain class for the same property passed after it, like a caller's className. */
function dropOverriddenThemeClasses(parts: string[], merged: string) {
  const key = parts.join("\u0000");
  const cached = themed.get(key);
  if (cached !== undefined) return cached;
  const dropped = new Set<string>();
  const decided = new Set<string>();
  const laterPlain: string[] = [];
  for (const part of [...parts].reverse()) {
    const classes = part.split(/\s+/).filter(Boolean);
    for (const className of classes.filter(isThemed)) {
      if (decided.has(className)) continue;
      decided.add(className);
      if (isOverridden(className, laterPlain)) dropped.add(className);
    }
    laterPlain.unshift(...classes.filter((className) => !isThemed(className)));
  }
  const result = merged
    .split(" ")
    .filter((className) => !dropped.has(className))
    .join(" ");
  themed.set(key, result);
  return result;
}

export function cn(...inputs: ClassValue[]) {
  const parts = inputs.map((input) => clsx(input)).filter(Boolean);
  const merged = twMerge(parts.join(" "));
  if (parts.length < 2 || !themeVariants.some((variant) => merged.includes(variant))) return merged;
  return dropOverriddenThemeClasses(parts, merged);
}
