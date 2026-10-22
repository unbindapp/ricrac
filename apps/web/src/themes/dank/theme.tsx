import { loadSymbols } from "@/components/icons/symbol-set";
import { DankNavigation, DankNavigationDrawer } from "@/themes/dank/navigation";
import { DankSettingsFrame } from "@/themes/dank/settings-frame";
import type { TUiTheme } from "@/themes/types";

export const dankTheme: TUiTheme = {
  id: "dank",
  label: "Dank",
  description: "Material 3 Expressive: bold color, round shapes and a navigation rail.",
  Navigation: DankNavigation,
  useIsNavigationHidden: () => false,
  NavigationDrawer: DankNavigationDrawer,
  SettingsFrame: DankSettingsFrame,
  icons: "symbols",
  createButton: "fab",
  separatesBars: true,
  threads: "drawer",
  defaultAccentHue: 257,
  load: loadSymbols,
};
