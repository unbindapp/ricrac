import { create } from "zustand";

const key = "ricrac:navigation-expanded";

function storedExpanded() {
  try {
    return localStorage.getItem(key) === "true";
  } catch {
    return false;
  }
}

function storeExpanded(isExpanded: boolean) {
  try {
    localStorage.setItem(key, String(isExpanded));
  } catch {
    return;
  }
}

type TSidebarStore = {
  isExpanded: boolean;
  toggle: () => void;
};

export const SIDEBAR_HOTKEY = "[";

/** The workspace navigation on wide screens: a rail, or expanded to the full sidebar. Narrow ones open it as a drawer. */
export const useSidebarStore = create<TSidebarStore>((set) => ({
  isExpanded: storedExpanded(),
  toggle: () =>
    set((state) => {
      storeExpanded(!state.isExpanded);
      return { isExpanded: !state.isExpanded };
    }),
}));
