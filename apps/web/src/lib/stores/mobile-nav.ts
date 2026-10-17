import { create } from "zustand";

type TMobileNavStore = {
  isOpen: boolean;
  setOpen: (isOpen: boolean) => void;
};

export const useMobileNavStore = create<TMobileNavStore>((set) => ({
  isOpen: false,
  setOpen: (isOpen) => set({ isOpen }),
}));
