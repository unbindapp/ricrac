import { DEFAULT_ACCENT_HUE } from "@/components/providers/themes";
import { SettingsLayout } from "@/components/layout/settings-layout";
import { SidebarNavigation, useIsSidebarHidden } from "@/components/workspace/sidebar-navigation";
import type { TUiTheme } from "@/themes/types";

export const defaultTheme: TUiTheme = {
  id: "default",
  label: "Default",
  description: "Calm and flat, with a sidebar.",
  Navigation: SidebarNavigation,
  useIsNavigationHidden: useIsSidebarHidden,
  SettingsFrame: ({ groups, children }) => (
    <SettingsLayout title="Settings" groups={groups}>
      {children}
    </SettingsLayout>
  ),
  icons: "lucide",
  defaultAccentHue: DEFAULT_ACCENT_HUE,
};
