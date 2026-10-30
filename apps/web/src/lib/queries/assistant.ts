import { useQueryClient } from "@tanstack/react-query";
import { useLocation } from "@tanstack/react-router";

import { toast } from "@/components/ui/toast";
import { queryKeyWorkspace } from "@/lib/queries/workspaces";
import type { TAssistantEvent } from "@/lib/server/assistant-events";
import { account } from "@/lib/server/client";
import { errorText } from "@/lib/server/errors";
import {
  emptyConversation,
  type TAssistantAnswer,
  type TAssistantPart,
  type TConversation,
  useAssistantStore,
} from "@/lib/stores/assistant";

const FAILED_TITLE = "Claude couldn't answer";

const runs = new Map<string, AbortController>();

function toastFailure(message: string) {
  toast.add({ type: "error", title: FAILED_TITLE, description: message });
}

function conversationOf(workspaceId: string) {
  return useAssistantStore.getState().conversations[workspaceId] ?? emptyConversation;
}

function withLastAnswer(
  conversation: TConversation,
  change: (answer: TAssistantAnswer) => TAssistantAnswer,
): TConversation {
  const last = conversation.messages.at(-1);
  if (last?.role !== "assistant") return conversation;
  return { ...conversation, messages: [...conversation.messages.slice(0, -1), change(last)] };
}

function withText(parts: TAssistantPart[], text: string): TAssistantPart[] {
  const last = parts.at(-1);
  if (last?.kind !== "text") return [...parts, { kind: "text", text }];
  return [...parts.slice(0, -1), { kind: "text", text: last.text + text }];
}

function applyEvent(conversation: TConversation, event: TAssistantEvent): TConversation {
  switch (event.event) {
    case "session":
      return { ...conversation, sessionId: event.data.sessionId };
    case "text":
      return withLastAnswer(conversation, (answer) => ({
        ...answer,
        parts: withText(answer.parts, event.data.text),
      }));
    case "tool":
      return withLastAnswer(conversation, (answer) => ({
        ...answer,
        parts: [...answer.parts, { kind: "tool", name: event.data.name }],
      }));
    case "done":
      return withLastAnswer(conversation, (answer) => ({ ...answer, done: event.data }));
    case "error":
      return withLastAnswer(conversation, (answer) => ({ ...answer, error: event.data.error }));
  }
}

const newAnswer = (): TAssistantAnswer => ({
  role: "assistant",
  parts: [],
  done: null,
  error: null,
  isStopped: false,
});

/** The conversation with Claude in a workspace, which lasts while the app is open. */
export function useAssistant(workspaceId: string) {
  const queryClient = useQueryClient();
  const page = useLocation({ select: (location) => location.pathname });
  const conversation = useAssistantStore(
    (state) => state.conversations[workspaceId] ?? emptyConversation,
  );
  const update = useAssistantStore((state) => state.update);

  const ask = (prompt: string) => {
    const text = prompt.trim();
    const { isRunning, sessionId } = conversationOf(workspaceId);
    if (!text || isRunning) return;

    const run = new AbortController();
    runs.set(workspaceId, run);
    let ranTool = false;
    update(workspaceId, (current) => ({
      ...current,
      isRunning: true,
      messages: [...current.messages, { role: "user", text }, newAnswer()],
    }));

    const onEvent = (event: TAssistantEvent) => {
      if (event.event === "tool") ranTool = true;
      if (event.event === "error") toastFailure(event.data.error);
      update(workspaceId, (current) => applyEvent(current, event));
    };

    void account
      .askAssistant(workspaceId, { prompt: text, sessionId, page }, onEvent, run.signal)
      .catch((error: unknown) => {
        if (run.signal.aborted) return;
        onEvent({ event: "error", data: { error: errorText(error) } });
      })
      .finally(() => {
        if (ranTool) {
          void queryClient.invalidateQueries({ queryKey: queryKeyWorkspace.all(workspaceId) });
        }
        if (runs.get(workspaceId) !== run) return;
        runs.delete(workspaceId);
        update(workspaceId, (current) => ({ ...current, isRunning: false }));
      });
  };

  const stop = () => {
    const run = runs.get(workspaceId);
    if (!run) return;
    update(workspaceId, (current) =>
      withLastAnswer(current, (answer) => ({ ...answer, isStopped: true })),
    );
    run.abort();
  };

  const reset = () => {
    runs.get(workspaceId)?.abort();
    runs.delete(workspaceId);
    update(workspaceId, () => emptyConversation);
  };

  return { ...conversation, ask, stop, reset };
}
