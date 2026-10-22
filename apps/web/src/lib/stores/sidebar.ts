import { create } from "zustand";

const key = "ricrac:sidebar-collapsed";

function storedCollapsed() {
  try {
    return localStorage.getItem(key) === "true";
  } catch {
    return false;
  }
}

function storeCollapsed(isCollapsed: boolean) {
  try {
    localStorage.setItem(key, String(isCollapsed));
  } catch {
    return;
  }
}

type TSidebarStore = {
  isCollapsed: boolean;
  toggle: () => void;
};

export const SIDEBAR_HOTKEY = "[";

export const useSidebarStore = create<TSidebarStore>((set) => ({
  isCollapsed: storedCollapsed(),
  toggle: () =>
    set((state) => {
      storeCollapsed(!state.isCollapsed);
      return { isCollapsed: !state.isCollapsed };
    }),
}));
