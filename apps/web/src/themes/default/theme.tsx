import { DEFAULT_ACCENT_HUE } from "@/components/providers/themes";
import { Sidebar } from "@/components/workspace/sidebar";
import { SidebarNavigation, useIsSidebarHidden } from "@/components/workspace/sidebar-navigation";
import type { TUiTheme } from "@/themes/types";

export const defaultTheme: TUiTheme = {
  id: "default",
  label: "Default",
  description: "Calm and flat, with a sidebar.",
  Navigation: SidebarNavigation,
  useIsNavigationHidden: useIsSidebarHidden,
  NavigationDrawer: Sidebar,
  icons: "lucide",
  createButton: "header",
  separatesBars: false,
  expandsReplies: false,
  chatBubbles: false,
  threadsInChat: false,
  defaultAccentHue: DEFAULT_ACCENT_HUE,
};
