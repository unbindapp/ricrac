import { loadSymbols } from "@/components/icons/symbol-set";
import { DankNavigation, DankNavigationDrawer } from "@/themes/dank/navigation";
import { DankSettingsFrame } from "@/themes/dank/settings-frame";
import { DankTopBar } from "@/themes/dank/top-bar";
import type { TUiTheme } from "@/themes/types";

export const dankTheme: TUiTheme = {
  id: "dank",
  label: "Dank",
  description: "Material 3 Expressive: bold color, round shapes and a navigation rail.",
  Navigation: DankNavigation,
  useIsNavigationHidden: () => false,
  TopBar: DankTopBar,
  NavigationDrawer: DankNavigationDrawer,
  SettingsFrame: DankSettingsFrame,
  icons: "symbols",
  createButton: "fab",
  separatesBars: true,
  expandsReplies: true,
  defaultAccentHue: 257,
  load: loadSymbols,
};
