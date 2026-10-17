import { create } from "zustand";

type TCommandPanelStore = {
  isOpen: boolean;
  setOpen: (isOpen: boolean) => void;
  isShortcutsOpen: boolean;
  setShortcutsOpen: (isOpen: boolean) => void;
};

export const useCommandPanelStore = create<TCommandPanelStore>((set) => ({
  isOpen: false,
  setOpen: (isOpen) => set({ isOpen }),
  isShortcutsOpen: false,
  setShortcutsOpen: (isShortcutsOpen) => set({ isShortcutsOpen }),
}));
