import { create } from "zustand";

import type { AssistantDone } from "@/api/account.gen";

export type TAssistantPart = { kind: "text"; text: string } | { kind: "tool"; name: string };

export type TAssistantAnswer = {
  role: "assistant";
  parts: TAssistantPart[];
  done: AssistantDone | null;
  error: string | null;
  isStopped: boolean;
};

export type TAssistantMessage = { role: "user"; text: string } | TAssistantAnswer;

export type TConversation = {
  messages: TAssistantMessage[];
  sessionId: string | null;
  isRunning: boolean;
};

export const emptyConversation: TConversation = { messages: [], sessionId: null, isRunning: false };

type TAssistantStore = {
  isOpen: boolean;
  setOpen: (isOpen: boolean) => void;
  toggle: () => void;
  /** By workspace id, kept while the app is open. */
  conversations: Record<string, TConversation>;
  update: (workspaceId: string, change: (conversation: TConversation) => TConversation) => void;
};

export const useAssistantStore = create<TAssistantStore>((set) => ({
  isOpen: false,
  setOpen: (isOpen) => set({ isOpen }),
  toggle: () => set((state) => ({ isOpen: !state.isOpen })),
  conversations: {},
  update: (workspaceId, change) =>
    set((state) => ({
      conversations: {
        ...state.conversations,
        [workspaceId]: change(state.conversations[workspaceId] ?? emptyConversation),
      },
    })),
}));
