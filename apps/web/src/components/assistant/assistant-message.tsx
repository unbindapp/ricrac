import type { AssistantDone } from "@/api/account.gen";
import { LoaderIcon, WrenchIcon } from "@/components/icons";
import { MarkdownView } from "@/components/shared/markdown-view";
import type { TAssistantAnswer } from "@/lib/stores/assistant";

const STOP_REASONS: Record<string, string> = {
  turns: "Stopped: hit the turn limit",
  budget: "Stopped: hit the budget",
  error: "Stopped: an error",
};

const MIN_SHOWN_COST_USD = 0.01;

function formatCost(costUsd: number) {
  if (costUsd > 0 && costUsd < MIN_SHOWN_COST_USD) return "<$0.01";
  return `$${costUsd.toFixed(2)}`;
}

function doneNotes({ costUsd, turns, stopped }: AssistantDone) {
  const notes = [formatCost(costUsd), `${turns} ${turns === 1 ? "turn" : "turns"}`];
  if (stopped) notes.push(STOP_REASONS[stopped] ?? `Stopped: ${stopped}`);
  return notes;
}

export function UserPrompt({ text }: { text: string }) {
  return (
    <article aria-label="Your question" data-side="me" className="flex justify-end pl-8">
      <div
        data-slot="message-bubble"
        className="bg-muted min-w-0 rounded-lg px-3 py-2 text-sm wrap-anywhere whitespace-pre-wrap"
      >
        {text}
      </div>
    </article>
  );
}

export function AssistantAnswer({
  answer,
  isRunning,
}: {
  answer: TAssistantAnswer;
  isRunning: boolean;
}) {
  const notes = answer.done ? doneNotes(answer.done) : answer.isStopped ? ["Stopped"] : [];
  return (
    <article aria-label="Claude's answer" aria-busy={isRunning} className="flex flex-col gap-2">
      {answer.parts.map((part, index) =>
        part.kind === "text" ? (
          <MarkdownView key={index} markdown={part.text} />
        ) : (
          <ToolLine key={index} name={part.name} />
        ),
      )}
      {isRunning && (
        <p className="text-muted-foreground flex items-center gap-1.5 text-xs">
          <LoaderIcon className="size-3.5 animate-spin" />
          Working…
        </p>
      )}
      {answer.error && (
        <p role="alert" className="text-destructive text-sm">
          {answer.error}
        </p>
      )}
      {notes.length > 0 && <p className="text-muted-foreground text-xs">{notes.join(" · ")}</p>}
    </article>
  );
}

function ToolLine({ name }: { name: string }) {
  return (
    <p className="text-muted-foreground flex items-center gap-1.5 text-xs">
      <WrenchIcon className="size-3.5" />
      <span className="min-w-0 truncate">
        Ran <span className="font-mono">{name}</span>
      </span>
    </p>
  );
}
