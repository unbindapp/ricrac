import { type Editor, Extension } from "@tiptap/core";
import type { Node as PMNode } from "@tiptap/pm/model";
import { Plugin, PluginKey } from "@tiptap/pm/state";
import { Decoration, DecorationSet } from "@tiptap/pm/view";

import { findQuote, type TTextRun } from "@/components/shared/editor/quote-ranges";

/** A passage to highlight: the first place its quote appears in the document. */
export type TAnnotation = { id: string; quote: string };

type TAnnotationsState = {
  annotations: TAnnotation[];
  activeId: string | null;
  decorations: DecorationSet;
};

const annotationsKey = new PluginKey<TAnnotationsState>("annotations");

const ANNOTATION_ID_ATTRIBUTE = "data-annotation-id";
const HIGHLIGHT_CLASS = "bg-warning/2-10 border-warning/6-10 cursor-pointer border-b-2";
const ACTIVE_HIGHLIGHT_CLASS = "bg-warning/4-10 border-warning cursor-pointer border-b-2";

type TAnnotationsOptions = { onActivate: { current: (id: string) => void } };

/** Highlights passages, set with `setAnnotations`; clicking one activates it. */
export const Annotations = Extension.create<TAnnotationsOptions>({
  name: "annotations",
  addOptions: () => ({ onActivate: { current: () => undefined } }),
  addProseMirrorPlugins() {
    const { onActivate } = this.options;
    return [
      new Plugin<TAnnotationsState>({
        key: annotationsKey,
        state: {
          init: () => ({ annotations: [], activeId: null, decorations: DecorationSet.empty }),
          apply: (tr, current, _oldState, newState) => {
            const next = tr.getMeta(annotationsKey) as
              Omit<TAnnotationsState, "decorations"> | undefined;
            if (next) return { ...next, decorations: decorate(newState.doc, next) };
            if (!tr.docChanged) return current;
            return { ...current, decorations: decorate(newState.doc, current) };
          },
        },
        props: {
          decorations: (state) => annotationsKey.getState(state)?.decorations,
          handleClick: (_view, _pos, event) => {
            const id = annotationIdAt(event.target);
            if (id) onActivate.current(id);
            return false;
          },
        },
      }),
    ];
  },
});

/** The highlight of an annotation, wherever its editor is. */
export function annotationElement(id: string) {
  return document.querySelector(`[${ANNOTATION_ID_ATTRIBUTE}="${CSS.escape(id)}"]`);
}

export function setAnnotations(
  editor: Editor,
  annotations: TAnnotation[],
  activeId: string | null,
) {
  if (editor.isDestroyed) return;
  const tr = editor.state.tr.setMeta(annotationsKey, { annotations, activeId });
  editor.view.dispatch(tr.setMeta("addToHistory", false));
}

function annotationIdAt(target: EventTarget | null) {
  if (!(target instanceof Element)) return null;
  return (
    target.closest(`[${ANNOTATION_ID_ATTRIBUTE}]`)?.getAttribute(ANNOTATION_ID_ATTRIBUTE) ?? null
  );
}

function decorate(doc: PMNode, { annotations, activeId }: Omit<TAnnotationsState, "decorations">) {
  if (annotations.length === 0) return DecorationSet.empty;
  const runs = textRuns(doc);
  const decorations = annotations.flatMap((annotation) => {
    const range = findQuote(runs, annotation.quote);
    if (!range) return [];
    const isActive = annotation.id === activeId;
    return [
      Decoration.inline(range.from, range.to, {
        class: isActive ? ACTIVE_HIGHLIGHT_CLASS : HIGHLIGHT_CLASS,
        [ANNOTATION_ID_ATTRIBUTE]: annotation.id,
      }),
    ];
  });
  return DecorationSet.create(doc, decorations);
}

function textRuns(doc: PMNode) {
  const runs: TTextRun[] = [];
  doc.descendants((node, pos) => {
    if (node.isText) runs.push({ text: node.text ?? "", from: pos });
    else if (node.isTextblock || node.isLeaf) runs.push({ text: "", from: null });
  });
  return runs;
}
