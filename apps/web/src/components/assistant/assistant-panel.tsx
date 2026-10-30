import { type KeyboardEvent, useState } from "react";

import { AssistantAnswer, UserPrompt } from "@/components/assistant/assistant-message";
import { useIsAssistantAvailable } from "@/components/assistant/use-assistant-available";
import {
  SendHorizontalIcon,
  SparklesIcon,
  SquareIcon,
  SquarePenIcon,
  XIcon,
} from "@/components/icons";
import { ChatLayout, useChatScroll } from "@/components/layout/chat-layout";
import { EmptyState } from "@/components/layout/empty-state";
import { PanelBar } from "@/components/layout/panel-bar";
import { useDeviceSize } from "@/components/providers/device-size-provider";
import { HeaderAction } from "@/components/shared/detail-actions";
import { Button } from "@/components/ui/button";
import { Drawer, DrawerContent, DrawerTitle } from "@/components/ui/drawer";
import { Textarea } from "@/components/ui/textarea";
import { SimpleTooltip } from "@/components/ui/tooltip";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { useAssistant } from "@/lib/queries/assistant";
import { useAssistantStore } from "@/lib/stores/assistant";

const EXAMPLE_PROMPTS = [
  "What's in progress this cycle?",
  "Find issues about login",
  "Label the untriaged bugs",
];

/** Claude beside the page on wide screens, in a drawer otherwise. */
export function AssistantPanel() {
  const isAvailable = useIsAssistantAvailable();
  const isOpen = useAssistantStore((state) => state.isOpen);
  const setOpen = useAssistantStore((state) => state.setOpen);
  const { size } = useDeviceSize();
  const close = () => setOpen(false);

  if (!isAvailable) return null;
  if (size !== "xl") {
    return (
      <Drawer direction="right" open={isOpen} onOpenChange={setOpen}>
        <DrawerContent className="w-96 max-w-full">
          <DrawerTitle className="sr-only">Claude</DrawerTitle>
          <AssistantChat onClose={close} />
        </DrawerContent>
      </Drawer>
    );
  }
  if (!isOpen) return null;
  return (
    <aside
      data-slot="split-pane"
      className="bg-background dank:border-0 flex w-96 shrink-0 flex-col overflow-hidden rounded-lg border"
    >
      <AssistantChat onClose={close} />
    </aside>
  );
}

function AssistantChat({ onClose }: { onClose: () => void }) {
  const { workspace } = useWorkspace();
  const { messages, isRunning, ask, stop, reset } = useAssistant(workspace.id);

  const header = (
    <PanelBar>
      <SparklesIcon className="text-muted-foreground size-4" />
      <h2 className="dank:type-title-medium min-w-0 flex-1 truncate text-sm font-medium">Claude</h2>
      <HeaderAction label="New conversation" onClick={reset}>
        <SquarePenIcon className="size-4" />
      </HeaderAction>
      <HeaderAction label="Close" onClick={onClose}>
        <XIcon className="size-4" />
      </HeaderAction>
    </PanelBar>
  );

  return (
    <section aria-label="Claude" className="bg-background flex min-h-0 w-full flex-1 flex-col">
      <ChatLayout
        header={header}
        composer={<AssistantComposer isRunning={isRunning} onAsk={ask} onStop={stop} />}
      >
        {messages.length === 0 ? (
          <EmptyState
            icon={SparklesIcon}
            title="Ask Claude"
            description="It can find, sum up and change issues, projects and cycles in this workspace."
            action={
              <div className="flex flex-col gap-2">
                {EXAMPLE_PROMPTS.map((prompt) => (
                  <Button key={prompt} variant="outline" size="sm" onClick={() => ask(prompt)}>
                    {prompt}
                  </Button>
                ))}
              </div>
            }
          />
        ) : (
          messages.map((message, index) =>
            message.role === "user" ? (
              <UserPrompt key={index} text={message.text} />
            ) : (
              <AssistantAnswer
                key={index}
                answer={message}
                isRunning={isRunning && index === messages.length - 1}
              />
            ),
          )
        )}
      </ChatLayout>
    </section>
  );
}

type TComposerProps = {
  isRunning: boolean;
  onAsk: (prompt: string) => void;
  onStop: () => void;
};

function AssistantComposer({ isRunning, onAsk, onStop }: TComposerProps) {
  const [prompt, setPrompt] = useState("");
  const { scrollToEnd } = useChatScroll();
  const canSend = prompt.trim() !== "" && !isRunning;

  const send = () => {
    if (!canSend) return;
    onAsk(prompt);
    setPrompt("");
    scrollToEnd();
  };

  const sendOnEnter = (event: KeyboardEvent<HTMLTextAreaElement>) => {
    if (event.key !== "Enter" || event.shiftKey || event.nativeEvent.isComposing) return;
    event.preventDefault();
    send();
  };

  return (
    <div className="flex flex-col gap-1.5">
      <div data-slot="composer" className="flex items-end gap-2">
        <Textarea
          aria-label="Ask Claude"
          placeholder="Ask about this workspace…"
          rows={1}
          autoFocus
          value={prompt}
          onChange={(event) => setPrompt(event.target.value)}
          onKeyDown={sendOnEnter}
          className="dank:min-h-12 dank:py-3 field-sizing-content max-h-40 min-h-10 min-w-0 flex-1 resize-none"
        />
        {isRunning ? (
          <SimpleTooltip content="Stop">
            <Button
              variant="outline"
              size="icon"
              aria-label="Stop"
              data-slot="composer-send"
              onClick={onStop}
            >
              <SquareIcon className="size-4" />
            </Button>
          </SimpleTooltip>
        ) : (
          <Button
            variant={canSend ? "default" : "ghost"}
            size="icon"
            aria-label="Send"
            data-slot="composer-send"
            disabled={!canSend}
            onClick={send}
          >
            <SendHorizontalIcon className="size-4" />
          </Button>
        )}
      </div>
      <p className="text-muted-foreground dank:hidden px-1 text-xs max-sm:hidden">
        Enter to send, Shift+Enter for a new line
      </p>
    </div>
  );
}
