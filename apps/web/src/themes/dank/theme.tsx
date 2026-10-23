import { loadSymbols } from "@/components/icons/symbol-set";
import { dankButtonVariants } from "@/themes/dank/button-variants";
import { LoadingIndicator } from "@/themes/dank/loading-indicator";
import { DankNavigation, DankNavigationDrawer } from "@/themes/dank/navigation";
import { DankSettingsWindow } from "@/themes/dank/dank-settings-window";
import { BASELINE_HUE, paintPalette } from "@/themes/dank/palette";
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
  SettingsWindow: DankSettingsWindow,
  LoadingIndicator,
  icons: "symbols",
  createButton: "fab",
  separatesBars: true,
  expandsReplies: true,
  defaultAccentHue: BASELINE_HUE,
  buttonVariants: dankButtonVariants,
  paintColors: paintPalette,
  load: loadSymbols,
};
