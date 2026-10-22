import { dankTheme } from "@/themes/dank/theme";
import { defaultTheme } from "@/themes/default/theme";
import { isUiThemeId, type TUiTheme, type TUiThemeId } from "@/themes/types";

export const uiThemes: Record<TUiThemeId, TUiTheme> = {
  default: defaultTheme,
  dank: dankTheme,
};

export function uiThemeOf(themeId: unknown): TUiTheme {
  return isUiThemeId(themeId) ? uiThemes[themeId] : defaultTheme;
}
