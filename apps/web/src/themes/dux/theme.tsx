import { loadSymbols } from "@/components/icons/symbol-set";
import { DuxAssistantSheet } from "@/themes/dux/assistant-sheet";
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
  description:
    "Material 3 Expressive in two tones: an app bar, a sidebar and pages on round cards.",
  Navigation: DuxNavigation,
  useIsNavigationHidden: () => false,
  TopBar: DuxTopBar,
  NavigationDrawer: DuxNavigationDrawer,
  SettingsWindow: DuxSettingsWindow,
  AssistantPanel: DuxAssistantSheet,
  ForumPanel: DuxForumPanel,
  LoadingIndicator,
  icons: "symbols",
  createButton: "header",
  separatesBars: true,
  inlineReplies: true,
  chatBubbles: true,
  threadsInChat: true,
  editorToolbars: false,
  pageTitles: true,
  showsEmptyPanes: false,
  mediaViewer: "stage",
  defaultAccentHue: BASELINE_HUE,
  buttonVariants: duxButtonVariants,
  paintColors: paintPalette,
  load: loadSymbols,
};
