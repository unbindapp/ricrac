import { useLocation } from "@tanstack/react-router";
import { useEffect } from "react";
import { create } from "zustand";

type TSettingsReturnStore = {
  href: string | null;
  remember: (href: string) => void;
};

/** Where closing the settings window goes back to. */
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

export function useSettingsReturnHref(workspaceUrlKey: string) {
  const href = useSettingsReturnStore((state) => state.href);
  if (!href) return null;
  const hrefWorkspace = href.split(/[/?#]/)[1]?.toLowerCase();
  if (hrefWorkspace !== workspaceUrlKey.toLowerCase()) return null;
  return href;
}
