import { useEffect } from "react";
import { create } from "zustand";

import type { TIssueDraft } from "@/components/issues/issue-draft";

/** What a new issue starts with: the properties the page or group implies. */
export type TNewIssueDefaults = {
  draft?: Partial<TIssueDraft>;
};

type TNewIssueStore = {
  isOpen: boolean;
  defaults: TNewIssueDefaults;
  context: TNewIssueDefaults;
  open: (defaults?: TNewIssueDefaults) => void;
  close: () => void;
  setContext: (context: TNewIssueDefaults) => void;
};

export const useNewIssueStore = create<TNewIssueStore>((set, get) => ({
  isOpen: false,
  defaults: {},
  context: {},
  open: (defaults = {}) => {
    const { context } = get();
    set({
      isOpen: true,
      defaults: { ...context, ...defaults, draft: { ...context.draft, ...defaults.draft } },
    });
  },
  close: () => set({ isOpen: false }),
  setContext: (context) => set({ context }),
}));

/** Lets the page's filters fill in new issues created from it (the C shortcut, the sidebar). */
export function useNewIssueContext(context: TNewIssueDefaults, enabled = true) {
  const setContext = useNewIssueStore((state) => state.setContext);
  const serialized = JSON.stringify(context);
  useEffect(() => {
    if (!enabled) return;
    setContext(JSON.parse(serialized) as TNewIssueDefaults);
    return () => setContext({});
  }, [serialized, setContext, enabled]);
}
