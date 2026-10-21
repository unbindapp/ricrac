import { defaultTheme, themes, TThemeWithoutSystem } from "@/components/providers/themes";
import { useAccentHueStore } from "@/lib/stores/accent-hue";
import { ThemeProvider as NextThemesProvider, ThemeProviderProps, useTheme } from "next-themes";
import * as React from "react";

export function ThemeProvider({ children, ...props }: ThemeProviderProps) {
  return (
    <NextThemesProvider
      attribute="class"
      themes={themes}
      defaultTheme={defaultTheme}
      disableTransitionOnChange
      {...props}
    >
      <ThemeProviderInner>{children}</ThemeProviderInner>
    </NextThemesProvider>
  );
}

function ThemeProviderInner({ children }: { children: React.ReactNode }) {
  const { theme, systemTheme } = useTheme();
  const finalTheme: TThemeWithoutSystem =
    (theme === "system" ? systemTheme : (theme as TThemeWithoutSystem)) || "dark";
  const hue = useAccentHueStore((state) => state.hue);

  React.useEffect(() => {
    // next-themes swaps the class in its own effect, after this one.
    const frame = requestAnimationFrame(() => {
      const metaThemeColor = document.querySelector('meta[name="theme-color"]');
      metaThemeColor?.setAttribute(
        "content",
        getComputedStyle(document.documentElement).backgroundColor,
      );
    });
    return () => cancelAnimationFrame(frame);
  }, [finalTheme, hue]);

  return children;
}
