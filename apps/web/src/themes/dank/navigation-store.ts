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

type TNavigationStore = {
  isExpanded: boolean;
  toggle: () => void;
};

/** The rail, or expanded to the full sidebar. */
export const useNavigationStore = create<TNavigationStore>((set) => ({
  isExpanded: storedExpanded(),
  toggle: () =>
    set((state) => {
      storeExpanded(!state.isExpanded);
      return { isExpanded: !state.isExpanded };
    }),
}));
