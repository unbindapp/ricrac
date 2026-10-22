import { useLocation } from "@tanstack/react-router";
import { useEffect } from "react";
import { create } from "zustand";

type TSettingsReturnStore = {
  href: string | null;
  remember: (href: string) => void;
};

/** The page a settings window opens over: the last one outside settings. */
export const useSettingsReturnStore = create<TSettingsReturnStore>((set) => ({
  href: null,
  remember: (href) => set({ href }),
}));

function isSettingsPath(pathname: string) {
  return pathname.split("/")[2] === "settings";
}

export function useRememberSettingsReturn() {
  const pathname = useLocation({ select: (location) => location.pathname });
  const searchStr = useLocation({ select: (location) => location.searchStr });
  const remember = useSettingsReturnStore((state) => state.remember);
  useEffect(() => {
    if (isSettingsPath(pathname)) return;
    remember(pathname + searchStr);
  }, [pathname, searchStr, remember]);
}

/** The page to open a settings page over, with the settings window showing it. */
export function settingsWindowHref(workspaceUrlKey: string, settingsPath: string) {
  const href = useSettingsReturnStore.getState().href;
  const hrefWorkspace = href?.split(/[/?#]/)[1]?.toLowerCase();
  const isInWorkspace = hrefWorkspace === workspaceUrlKey.toLowerCase();
  const url = new URL(
    href && isInWorkspace ? href : `/${workspaceUrlKey}/issues`,
    window.location.origin,
  );
  url.searchParams.set("settings", settingsPath);
  return url.pathname + url.search;
}
