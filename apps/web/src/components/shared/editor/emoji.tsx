import { Extension, type Editor, type Range } from "@tiptap/core";
import { PluginKey } from "@tiptap/pm/state";
import { Suggestion } from "@tiptap/suggestion";

import {
  suggestionRenderer,
  type TSuggestionSink,
  type TSuggestionStateRef,
} from "@/components/shared/editor/suggestions";
import type { TSuggestionItem } from "@/components/ui/suggestion-menu";

const MAX_RESULTS = 8;
const MIN_QUERY = 2;

type TEmoji = { char: string; name: string; names: string[] };
type TEmojiItem = TSuggestionItem & { char: string };

let emojis: Promise<TEmoji[]> | null = null;

function loadEmojis() {
  emojis ??= import("emoji-picker-react/data/emojis-en").then(({ default: data }) =>
    Object.entries(data.emojis)
      .filter(([category]) => category !== "custom")
      .flatMap(([, list]) =>
        list.map((emoji) => ({
          char: String.fromCodePoint(...emoji.u.split("-").map((code) => parseInt(code, 16))),
          name: emoji.n[emoji.n.length - 1],
          names: emoji.n,
        })),
      ),
  );
  return emojis;
}

async function matchEmojis(query: string): Promise<TEmojiItem[]> {
  const needle = query.toLowerCase().replace(/_/g, " ");
  if (needle.length < MIN_QUERY || needle.includes(" ")) return [];
  const all = await loadEmojis();
  const rank = (emoji: TEmoji) => {
    if (emoji.names.includes(needle)) return 0;
    if (emoji.name.startsWith(needle)) return 1;
    if (emoji.names.some((name) => name.startsWith(needle))) return 2;
    return emoji.names.some((name) => name.includes(needle)) ? 3 : 4;
  };
  return all
    .map((emoji) => ({ emoji, rank: rank(emoji) }))
    .filter(({ rank }) => rank < 4)
    .sort((a, b) => a.rank - b.rank)
    .slice(0, MAX_RESULTS)
    .map(({ emoji }) => ({
      id: emoji.char,
      char: emoji.char,
      label: emoji.name,
      icon: <span className="w-4 text-center text-sm leading-none">{emoji.char}</span>,
    }));
}

type TEmojiOptions = { sink: TSuggestionSink; state: TSuggestionStateRef };

export const EmojiSuggestions = Extension.create<TEmojiOptions>({
  name: "emojiSuggestions",
  addProseMirrorPlugins() {
    const { sink, state } = this.options;
    return [
      Suggestion<TEmojiItem, TEmojiItem>({
        editor: this.editor,
        pluginKey: new PluginKey("emojiSuggestions"),
        char: ":",
        allow: ({ editor }) => !editor.isActive("codeBlock") && !editor.isActive("code"),
        items: ({ query }) => matchEmojis(query),
        command: ({
          editor,
          range,
          props,
        }: {
          editor: Editor;
          range: Range;
          props: TEmojiItem;
        }) => {
          editor.chain().focus().deleteRange(range).insertContent(props.char).run();
        },
        render: suggestionRenderer<TEmojiItem>(sink, state, "No matching emoji", {
          hideWhenEmpty: true,
        }),
      }),
    ];
  },
});
