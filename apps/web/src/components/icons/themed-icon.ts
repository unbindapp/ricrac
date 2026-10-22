import type { LucideIcon, LucideProps } from "lucide-react";
import { createElement, forwardRef, useContext } from "react";

import { SymbolSetContext } from "./symbol-set.ts";

const FILL_CLASS = /^(?:data-([\w-]+):)?fill-(?!none\b|transparent\b)/;

/** Whether a lucide user meant the icon filled: `fill-current`, or `data-x:fill-*` while data-x is set. */
function isFilled(className: string | undefined, props: Record<string, unknown>) {
  if (!className) return false;
  return className.split(/\s+/).some((token) => {
    const match = FILL_CLASS.exec(token);
    if (!match) return false;
    const [, dataAttribute] = match;
    return !dataAttribute || Boolean(props[`data-${dataAttribute}`]);
  });
}

/** A lucide icon that draws its Material Symbols Rounded glyph while a symbol set is provided. */
export function themedIcon(Lucide: LucideIcon, name: string): LucideIcon {
  const Icon = forwardRef<SVGSVGElement, LucideProps>(function ThemedIcon(props, ref) {
    const paths = useContext(SymbolSetContext)?.[name];
    if (!paths) return createElement(Lucide, { ...props, ref });
    const { size = 24, color = "currentColor", children, ...rest } = props;
    delete rest.strokeWidth;
    delete rest.absoluteStrokeWidth;
    const [outline, filled = outline] = paths;
    return createElement(
      "svg",
      {
        ref,
        xmlns: "http://www.w3.org/2000/svg",
        viewBox: "0 -960 960 960",
        width: size,
        height: size,
        fill: color,
        "aria-hidden": rest["aria-label"] ? undefined : true,
        ...rest,
      },
      createElement("path", { d: isFilled(rest.className, rest) ? filled : outline }),
      children,
    );
  });
  Icon.displayName = name;
  return Icon;
}
