import { loadSymbols } from "@/components/icons/symbol-set";
import { DankNavigation } from "@/themes/dank/navigation";
import { DankSettingsFrame } from "@/themes/dank/settings-frame";
import type { TUiTheme } from "@/themes/types";

export const dankTheme: TUiTheme = {
  id: "dank",
  label: "Dank",
  description: "Material 3 Expressive: bold color, round shapes and a navigation rail.",
  Navigation: DankNavigation,
  useIsNavigationHidden: () => false,
  SettingsFrame: DankSettingsFrame,
  icons: "symbols",
  defaultAccentHue: 257,
  load: loadSymbols,
};
