import type { ComponentType, ReactNode } from "react";

import type { TSettingsTabGroup } from "@/components/layout/settings-layout";

export const UI_THEME_IDS = ["default", "dank"] as const;
export type TUiThemeId = (typeof UI_THEME_IDS)[number];

export type TSettingsFrameProps = { groups: TSettingsTabGroup[]; children: ReactNode };

/** A look for the whole app: its tokens and component styles live in CSS under [data-ui-theme=id]. */
export type TUiTheme = {
  id: TUiThemeId;
  label: string;
  description: string;
  /** The navigation beside pages on wide screens; it owns its toggle hotkey. */
  Navigation: ComponentType;
  /** True while the navigation is hidden and pages offer a button to show it. */
  useIsNavigationHidden: () => boolean;
  /** Sits above pages on wide screens, like a search bar. */
  TopBar?: ComponentType;
  /** The navigation in the drawer on narrow screens. */
  NavigationDrawer: ComponentType<{ onNavigate: () => void }>;
  /** Frames the settings pages with their tabs. */
  SettingsFrame: ComponentType<TSettingsFrameProps>;
  icons: "lucide" | "symbols";
  /** Where a page's button for creating its things goes: its header, or floating bottom right. */
  createButton: "header" | "fab";
  /** Page and panel bars sit on their own surface, apart from what scrolls under them. */
  separatesBars: boolean;
  /** Replies show under the message they answer instead of behind a count. */
  expandsReplies: boolean;
  /** The accent hue until someone picks their own; the theme's CSS sets the same --accent-hue. */
  defaultAccentHue: number;
  /** Fetches what the theme needs before it shows, like its icon set. */
  load?: () => Promise<void>;
};

export function isUiThemeId(value: unknown): value is TUiThemeId {
  return UI_THEME_IDS.includes(value as TUiThemeId);
}
