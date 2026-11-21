import { loadSymbols } from "@/components/icons/symbol-set";
import { duxButtonVariants } from "@/themes/dux/button-variants";
import { LoadingIndicator } from "@/themes/dux/loading-indicator";
import { DuxNavigation, DuxNavigationDrawer } from "@/themes/dux/navigation";
import { DuxSettingsWindow } from "@/themes/dux/dux-settings-window";
import { DuxForumPanel } from "@/themes/dux/forum-panel";
import { BASELINE_HUE, paintPalette } from "@/themes/dux/palette";
import { DuxTopBar } from "@/themes/dux/top-bar";
import type { TUiTheme } from "@/themes/types";

export const duxTheme: TUiTheme = {
  id: "dux",
  label: "Dux",
  description: "Material 3 Expressive: bold color, round shapes and a navigation rail.",
  Navigation: DuxNavigation,
  useIsNavigationHidden: () => false,
  TopBar: DuxTopBar,
  NavigationDrawer: DuxNavigationDrawer,
  SettingsWindow: DuxSettingsWindow,
  ForumPanel: DuxForumPanel,
  LoadingIndicator,
  icons: "symbols",
  createButton: "fab",
  separatesBars: true,
  inlineReplies: true,
  chatBubbles: true,
  threadsInChat: true,
  editorToolbars: false,
  pageTitles: false,
  showsEmptyPanes: false,
  mediaViewer: "stage",
  defaultAccentHue: BASELINE_HUE,
  buttonVariants: duxButtonVariants,
  paintColors: paintPalette,
  load: loadSymbols,
};
