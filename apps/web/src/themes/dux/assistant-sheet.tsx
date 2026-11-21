import { type KeyboardEvent, useEffect, useRef, useState } from "react";

import { AssistantAnswer, UserPrompt } from "@/components/assistant/assistant-message";
import { useIsAssistantAvailable } from "@/components/assistant/use-assistant-available";
import { ClaudeIcon } from "@/components/icons/claude";
import { SendIcon, SquareIcon, SquarePenIcon, XIcon } from "@/components/icons";
import { Button } from "@/components/ui/button";
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

/** Claude in a side sheet: docked beside the page on wide screens, over it on narrow ones. */
export function DuxAssistantSheet() {
  const isAvailable = useIsAssistantAvailable();
  const isOpen = useAssistantStore((state) => state.isOpen);
  const setOpen = useAssistantStore((state) => state.setOpen);
  if (!isAvailable || !isOpen) return null;
  return (
    <aside
      role="dialog"
      aria-labelledby="dux-ask-title"
      data-slot="dux-sheet"
      onKeyDown={(event) => {
        if (event.key !== "Escape") return;
        event.preventDefault();
        setOpen(false);
      }}
    >
      <AssistantChat onClose={() => setOpen(false)} />
    </aside>
  );
}

function AssistantChat({ onClose }: { onClose: () => void }) {
  const { workspace } = useWorkspace();
  const { messages, isRunning, ask, stop, reset } = useAssistant(workspace.id);
  const [prompt, setPrompt] = useState("");
  const inputRef = useRef<HTMLTextAreaElement>(null);
  const scrollRef = useRef<HTMLDivElement>(null);
  const canSend = prompt.trim() !== "" && !isRunning;

  useEffect(() => {
    const scroller = scrollRef.current;
    if (scroller) scroller.scrollTop = scroller.scrollHeight;
  }, [messages]);

  const send = () => {
    if (!canSend) return;
    ask(prompt);
    setPrompt("");
    inputRef.current?.focus();
  };

  const sendOnEnter = (event: KeyboardEvent<HTMLTextAreaElement>) => {
    if (event.key !== "Enter" || event.shiftKey || event.nativeEvent.isComposing) return;
    event.preventDefault();
    send();
  };

  const suggest = (example: string) => {
    setPrompt(example);
    inputRef.current?.focus();
  };

  return (
    <>
      <div data-slot="dux-sheet-head">
        <span data-slot="dux-sheet-title">
          <ClaudeIcon />
          Claude
        </span>
        {messages.length > 0 && (
          <SimpleTooltip content="New conversation" side="bottom">
            <Button
              variant="ghost"
              size="icon"
              forceMinSize={false}
              aria-label="New conversation"
              onClick={reset}
            >
              <SquarePenIcon />
            </Button>
          </SimpleTooltip>
        )}
        <SimpleTooltip content="Close" side="bottom">
          <Button
            variant="ghost"
            size="icon"
            forceMinSize={false}
            aria-label="Close"
            onClick={onClose}
          >
            <XIcon />
          </Button>
        </SimpleTooltip>
      </div>
      <div data-slot="dux-sheet-main">
        <div ref={scrollRef} data-slot="dux-sheet-body">
          {messages.length === 0 ? (
            <>
              <h2 id="dux-ask-title">Ask Claude about your workspace</h2>
              <p data-slot="dux-sheet-description">
                It can find, sum up and change issues, projects and cycles in this workspace.
              </p>
              <div data-slot="dux-sheet-suggestions">
                {EXAMPLE_PROMPTS.map((example) => (
                  <Button
                    key={example}
                    variant="card"
                    forceMinSize={false}
                    data-slot="dux-suggestion"
                    onClick={() => suggest(example)}
                  >
                    {example}
                  </Button>
                ))}
              </div>
            </>
          ) : (
            <div data-slot="dux-sheet-messages">
              <h2 id="dux-ask-title" className="sr-only">
                Claude
              </h2>
              {messages.map((message, index) =>
                message.role === "user" ? (
                  <UserPrompt key={index} text={message.text} />
                ) : (
                  <AssistantAnswer
                    key={index}
                    answer={message}
                    isRunning={isRunning && index === messages.length - 1}
                  />
                ),
              )}
            </div>
          )}
        </div>
        <form
          data-slot="dux-sheet-composer"
          onSubmit={(event) => {
            event.preventDefault();
            send();
          }}
        >
          <div data-slot="dux-composer-row">
            <Textarea
              ref={inputRef}
              aria-label="Message Claude"
              placeholder="Ask Claude"
              rows={1}
              autoFocus
              value={prompt}
              onChange={(event) => setPrompt(event.target.value)}
              onKeyDown={sendOnEnter}
            />
            {isRunning ? (
              <SimpleTooltip content="Stop">
                <Button
                  variant="default"
                  size="icon"
                  forceMinSize={false}
                  aria-label="Stop"
                  data-slot="dux-send"
                  onClick={stop}
                >
                  <SquareIcon className="fill-current" />
                </Button>
              </SimpleTooltip>
            ) : (
              <Button
                type="submit"
                variant="default"
                size="icon"
                forceMinSize={false}
                aria-label="Send"
                data-slot="dux-send"
                disabled={!canSend}
              >
                <SendIcon className="fill-current" />
              </Button>
            )}
          </div>
        </form>
      </div>
    </>
  );
}
