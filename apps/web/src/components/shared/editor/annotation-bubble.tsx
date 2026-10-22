import type { Editor } from "@tiptap/core";
import type { EditorState } from "@tiptap/pm/state";
import { BubbleMenu, type BubbleMenuProps } from "@tiptap/react/menus";
import { useEffect, useRef } from "react";
import type { LucideIcon } from "lucide-react";

import { Button } from "@/components/ui/button";

export type TAnnotationAction = {
  label: string;
  icon: LucideIcon;
  /** Gets the selected text. */
  onSelect: (quote: string) => void;
};

const BUBBLE_OPTIONS: BubbleMenuProps["options"] = { placement: "top", offset: 6 };

const hasSelectedText: BubbleMenuProps["shouldShow"] = ({ editor, view, state, element }) => {
  if (!selectedText(state)) return false;
  if (!editor.isEditable) return true;
  return view.hasFocus() || element.contains(document.activeElement);
};

/** Actions on the selected text, shown beside it; read-only editors show them too. */
export function AnnotationBubble({
  editor,
  actions,
}: {
  editor: Editor;
  actions: TAnnotationAction[];
}) {
  const bubbleRef = useRef<HTMLDivElement>(null);
  useCollapseWhenSelectionLeaves(editor, bubbleRef);
  return (
    <BubbleMenu
      ref={bubbleRef}
      editor={editor}
      pluginKey="annotationBubble"
      options={BUBBLE_OPTIONS}
      shouldShow={hasSelectedText}
      className="bg-popover text-popover-foreground z-50 flex items-center gap-0.5 rounded-md border p-0.5 shadow-md"
    >
      {actions.map(({ label, icon: Icon, onSelect }) => (
        <Button
          key={label}
          variant="ghost"
          size="sm"
          forceMinSize={false}
          onMouseDown={(event) => event.preventDefault()}
          onClick={() => {
            const quote = selectedText(editor.state);
            editor.commands.setTextSelection(editor.state.selection.to);
            onSelect(quote);
          }}
        >
          <Icon className="size-4" />
          {label}
        </Button>
      ))}
    </BubbleMenu>
  );
}

function selectedText(state: EditorState) {
  const { from, to, empty } = state.selection;
  if (empty) return "";
  return state.doc.textBetween(from, to, "\n\n", " ").trim();
}

/** A read-only editor keeps its selection when the page's moves elsewhere, which would leave the bubble up. */
function useCollapseWhenSelectionLeaves(
  editor: Editor,
  bubbleRef: React.RefObject<HTMLDivElement | null>,
) {
  useEffect(() => {
    const collapse = () => {
      if (editor.isDestroyed || editor.isEditable || editor.state.selection.empty) return;
      if (bubbleRef.current?.contains(document.activeElement)) return;
      const selection = window.getSelection();
      const isInEditor = selection?.anchorNode && editor.view.dom.contains(selection.anchorNode);
      if (isInEditor && !selection.isCollapsed) return;
      editor.commands.setTextSelection(editor.state.selection.to);
    };
    document.addEventListener("selectionchange", collapse);
    return () => document.removeEventListener("selectionchange", collapse);
  }, [editor, bubbleRef]);
}
