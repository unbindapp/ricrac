import type {
  AssistantDone,
  AssistantSession,
  AssistantText,
  AssistantTool,
} from "@/api/account.gen";

export type TAssistantEvent =
  | { event: "session"; data: AssistantSession }
  | { event: "text"; data: AssistantText }
  | { event: "tool"; data: AssistantTool }
  | { event: "done"; data: AssistantDone }
  | { event: "error"; data: { error: string } };

const EVENT_NAMES = new Set<string>(["session", "text", "tool", "done", "error"]);

function fieldValue(line: string, field: string) {
  const value = line.slice(field.length + 1);
  return value.startsWith(" ") ? value.slice(1) : value;
}

function parseEvent(block: string): TAssistantEvent | null {
  let name = "";
  const data: string[] = [];
  for (const line of block.split("\n")) {
    if (line.startsWith("event:")) name = fieldValue(line, "event");
    if (line.startsWith("data:")) data.push(fieldValue(line, "data"));
  }
  if (!EVENT_NAMES.has(name) || data.length === 0) return null;
  return { event: name, data: JSON.parse(data.join("\n")) } as TAssistantEvent;
}

/** The events complete in a server-sent event stream, and the text after them still arriving. */
export function parseAssistantEvents(buffer: string): { events: TAssistantEvent[]; rest: string } {
  const blocks = buffer.split("\n\n");
  const rest = blocks.pop() ?? "";
  const events = blocks.map(parseEvent).filter((event) => event !== null);
  return { events, rest };
}
