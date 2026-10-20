import type { SuggestionOptions, SuggestionProps } from "@tiptap/suggestion";

import type { TSuggestionItem } from "@/components/ui/suggestion-menu";

export type TSuggestionState = {
  items: TSuggestionItem[];
  index: number;
  rect: DOMRect | null;
  emptyText: string;
  command: (item: TSuggestionItem) => void;
};

export type TSuggestionSink = { current: (next: TSuggestionState | null) => void };
export type TSuggestionStateRef = { current: TSuggestionState | null };

export function suggestionRenderer<T extends TSuggestionItem>(
  sink: TSuggestionSink,
  state: TSuggestionStateRef,
  emptyText: string,
  {
    hideWhenEmpty = false,
    toProps,
  }: { hideWhenEmpty?: boolean; toProps?: (item: T) => unknown } = {},
): SuggestionOptions<T>["render"] {
  const show = (props: SuggestionProps<T>) => {
    if (hideWhenEmpty && props.items.length === 0) {
      sink.current(null);
      return;
    }
    sink.current({
      items: props.items,
      index: 0,
      rect: props.clientRect?.() ?? null,
      emptyText,
      command: (item) => props.command((toProps ? toProps(item as T) : item) as T),
    });
  };
  return () => ({
    onStart: show,
    onUpdate: show,
    onKeyDown: ({ event }) => handleSuggestionKey(event, state.current, sink.current),
    onExit: () => sink.current(null),
  });
}

function handleSuggestionKey(
  event: KeyboardEvent,
  state: TSuggestionState | null,
  update: (next: TSuggestionState | null) => void,
) {
  if (!state) return false;
  const count = state.items.length;
  switch (event.key) {
    case "ArrowDown":
      if (count === 0) return true;
      update({ ...state, index: (state.index + 1) % count });
      return true;
    case "ArrowUp":
      if (count === 0) return true;
      update({ ...state, index: (state.index - 1 + count) % count });
      return true;
    case "Enter":
    case "Tab": {
      const item = state.items[state.index];
      if (!item) return false;
      state.command(item);
      return true;
    }
    case "Escape":
      update(null);
      return true;
    default:
      return false;
  }
}
