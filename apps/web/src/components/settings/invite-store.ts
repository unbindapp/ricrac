import { create } from "zustand";

type TInviteStore = {
  isOpen: boolean;
  setOpen: (isOpen: boolean) => void;
};

/** The invite dialog, opened from the workspace menu, the command panel and member settings. */
export const useInviteStore = create<TInviteStore>((set) => ({
  isOpen: false,
  setOpen: (isOpen) => set({ isOpen }),
}));
