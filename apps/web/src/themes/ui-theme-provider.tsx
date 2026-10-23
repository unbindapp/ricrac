import { createContext, type ReactNode, use, useContext, useLayoutEffect } from "react";

import { loadedSymbols, SymbolSetContext } from "@/components/icons/symbol-set";
import { useAccentHueStore } from "@/lib/stores/accent-hue";
import { defaultTheme } from "@/themes/default/theme";
import { uiThemeOf } from "@/themes/registry";
import type { TUiTheme } from "@/themes/types";

const UiThemeContext = createContext<TUiTheme>(defaultTheme);

const loads = new Map<string, Promise<void>>();

function loaded(theme: TUiTheme) {
  if (!theme.load) return undefined;
  let promise = loads.get(theme.id);
  if (!promise) {
    promise = theme.load();
    loads.set(theme.id, promise);
  }
  return promise;
}

export function uiThemeStorageKey(workspaceUrlKey: string) {
  return `ricrac:ui-theme:${workspaceUrlKey}`;
}

function remember(workspaceUrlKey: string, themeId: string) {
  try {
    localStorage.setItem(uiThemeStorageKey(workspaceUrlKey), themeId);
  } catch {
    return;
  }
}

/** Applies a workspace's theme to the whole page while the workspace is open. */
export function UiThemeProvider({
  themeId,
  workspaceUrlKey,
  children,
}: {
  themeId: string | null | undefined;
  workspaceUrlKey: string;
  children: ReactNode;
}) {
  const theme = uiThemeOf(themeId);
  const accentHue = useAccentHueStore((state) => state.hue);
  const pending = loaded(theme);
  if (pending) use(pending);

  useLayoutEffect(() => {
    const root = document.documentElement;
    root.dataset.uiTheme = theme.id;
    remember(workspaceUrlKey, theme.id);
    return () => {
      delete root.dataset.uiTheme;
    };
  }, [theme.id, workspaceUrlKey]);

  useLayoutEffect(() => theme.paintColors?.(accentHue), [theme, accentHue]);

  return (
    <UiThemeContext value={theme}>
      <SymbolSetContext value={theme.icons === "symbols" ? loadedSymbols() : null}>
        {children}
      </SymbolSetContext>
    </UiThemeContext>
  );
}

export function useUiTheme() {
  return useContext(UiThemeContext);
}
