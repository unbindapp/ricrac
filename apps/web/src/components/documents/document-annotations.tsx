import type { Editor } from "@tiptap/core";
import { useEffect } from "react";

import { MessageSquarePlusIcon, PencilLineIcon } from "@/components/icons";
import {
  AnnotationBubble,
  type TAnnotationAction,
} from "@/components/shared/editor/annotation-bubble";
import { setAnnotations, type TAnnotation } from "@/components/shared/editor/annotations";

/** A comment or suggestion being written about a passage of the document. */
export type TCommentDraft = { quotedText: string; suggest: boolean };

type TProps = {
  editor: Editor;
  annotations: TAnnotation[];
  activeId: string | null;
  canSuggest: boolean;
  onAnnotate: (draft: TCommentDraft) => void;
};

/** Highlights the passages threads quote and offers to comment on or suggest a change to a selection. */
export function DocumentAnnotations({
  editor,
  annotations,
  activeId,
  canSuggest,
  onAnnotate,
}: TProps) {
  useEffect(() => {
    setAnnotations(editor, annotations, activeId);
  }, [editor, annotations, activeId]);

  const actions: TAnnotationAction[] = [
    {
      label: "Comment",
      icon: MessageSquarePlusIcon,
      onSelect: (quotedText) => onAnnotate({ quotedText, suggest: false }),
    },
  ];
  if (canSuggest) {
    actions.push({
      label: "Suggest edit",
      icon: PencilLineIcon,
      onSelect: (quotedText) => onAnnotate({ quotedText, suggest: true }),
    });
  }
  return <AnnotationBubble editor={editor} actions={actions} />;
}
