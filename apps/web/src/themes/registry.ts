import { dankTheme } from "@/themes/dank/theme";
import { defaultTheme } from "@/themes/default/theme";
import type { TUiTheme, TUiThemeId } from "@/themes/types";

export const uiThemes: Record<TUiThemeId, TUiTheme> = {
  default: defaultTheme,
  dank: dankTheme,
};
