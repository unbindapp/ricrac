import type { ComponentType } from "react";

import type { buttonVariants } from "@/components/ui/button";

export const UI_THEME_IDS = ["default", "dank"] as const;
export type TUiThemeId = (typeof UI_THEME_IDS)[number];

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
  /** Shows while a page loads, in place of the spinner. */
  LoadingIndicator?: ComponentType;
  /** Opens settings in a window over the page, which stays as it was, instead of as pages. */
  SettingsWindow?: ComponentType;
  icons: "lucide" | "symbols";
  /** Where a page's button for creating its things goes: its header, or floating bottom right. */
  createButton: "header" | "fab";
  /** Page and panel bars sit on their own surface, apart from what scrolls under them. */
  separatesBars: boolean;
  /** Replies show under the message they answer instead of behind a count. */
  expandsReplies: boolean;
  /** The accent hue until someone picks their own; the theme's CSS sets the same --accent-hue. */
  defaultAccentHue: number;
  /** The theme's own button styles, with the same variants and sizes. */
  buttonVariants?: typeof buttonVariants;
  /** Sets the colors the theme derives from the accent hue in script, where CSS can't. */
  paintColors?: (accentHue: number | null) => void;
  /** Fetches what the theme needs before it shows, like its icon set. */
  load?: () => Promise<void>;
};

export function isUiThemeId(value: unknown): value is TUiThemeId {
  return UI_THEME_IDS.includes(value as TUiThemeId);
}
