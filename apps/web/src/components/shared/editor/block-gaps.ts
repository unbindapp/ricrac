import { Extension } from "@tiptap/core";
import type { Node } from "@tiptap/pm/model";
import { Plugin, PluginKey, TextSelection } from "@tiptap/pm/state";
import { Decoration, DecorationSet, type EditorView } from "@tiptap/pm/view";

const isText = (node: Node | null) => node?.isTextblock ?? false;

/** Two blocks you can't type between, like stacked images, or an image at either end. */
const needsGap = (before: Node | null, after: Node | null) =>
  !isText(before) && !isText(after) && Boolean(before?.isAtom || after?.isAtom);

function writeAt(view: EditorView, getPos: () => number | undefined) {
  const pos = getPos();
  if (pos === undefined || !view.editable) return;
  const { state } = view;
  const tr = state.tr.insert(pos, state.schema.nodes.paragraph.create());
  view.dispatch(tr.setSelection(TextSelection.create(tr.doc, pos + 1)).scrollIntoView());
  view.focus();
}

function gapWidget(view: EditorView, getPos: () => number | undefined) {
  const gap = document.createElement("div");
  gap.className = "rr-block-gap";
  gap.contentEditable = "false";
  gap.title = "Click to write here";
  gap.addEventListener("mousedown", (event) => {
    if (event.button !== 0) return;
    event.preventDefault();
    writeAt(view, getPos);
  });
  return gap;
}

const gapSpec = { ignoreSelection: true, stopEvent: (event: Event) => event.type === "mousedown" };
const gapBefore = { ...gapSpec, side: -1 };
const gapAfter = { ...gapSpec, side: 1 };

function gapDecorations(doc: Node) {
  const gaps: Decoration[] = [];
  let pos = 0;
  for (let index = 0; index <= doc.childCount; index += 1) {
    const before = index > 0 ? doc.child(index - 1) : null;
    const after = index < doc.childCount ? doc.child(index) : null;
    if (needsGap(before, after)) {
      gaps.push(Decoration.widget(pos, gapWidget, after ? gapBefore : gapAfter));
    }
    pos += after?.nodeSize ?? 0;
  }
  return DecorationSet.create(doc, gaps);
}

/** Room to click between blocks that have no text, where a click starts a paragraph. */
export const BlockGaps = Extension.create({
  name: "blockGaps",
  addProseMirrorPlugins() {
    const editor = this.editor;
    return [
      new Plugin({
        key: new PluginKey("blockGaps"),
        props: {
          decorations: (state) => (editor.isEditable ? gapDecorations(state.doc) : null),
        },
      }),
    ];
  },
});
