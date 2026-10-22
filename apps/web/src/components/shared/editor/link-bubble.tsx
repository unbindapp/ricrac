import type { Editor } from "@tiptap/core";
import { useEditorState } from "@tiptap/react";
import { BubbleMenu } from "@tiptap/react/menus";

import { ExternalLinkIcon, PencilIcon, UnlinkIcon } from "@/components/icons/symbols.gen";
import { Button } from "@/components/ui/button";
import { SimpleTooltip } from "@/components/ui/tooltip";

export function LinkBubble({ editor, onEdit }: { editor: Editor; onEdit: () => void }) {
  const href = useEditorState({
    editor,
    selector: ({ editor: current }) => (current.getAttributes("link").href as string) ?? "",
  });
  return (
    <BubbleMenu
      editor={editor}
      pluginKey="linkBubble"
      options={{ placement: "bottom-start", offset: 6 }}
      shouldShow={({ editor: current, state }) =>
        current.isEditable && state.selection.empty && current.isActive("link")
      }
      className="bg-popover text-popover-foreground z-50 flex max-w-sm items-center gap-0.5 rounded-md border p-0.5 shadow-md"
    >
      <a
        href={href}
        target="_blank"
        rel="noreferrer noopener"
        className="text-muted-foreground min-w-0 truncate px-2 text-xs"
      >
        {href}
      </a>
      <BubbleButton label="Open link" onClick={() => window.open(href, "_blank", "noopener")}>
        <ExternalLinkIcon className="size-4" />
      </BubbleButton>
      <BubbleButton label="Edit link" onClick={onEdit}>
        <PencilIcon className="size-4" />
      </BubbleButton>
      <BubbleButton
        label="Remove link"
        onClick={() => editor.chain().focus().extendMarkRange("link").unsetLink().run()}
      >
        <UnlinkIcon className="size-4" />
      </BubbleButton>
    </BubbleMenu>
  );
}

function BubbleButton({
  label,
  onClick,
  children,
}: {
  label: string;
  onClick: () => void;
  children: React.ReactNode;
}) {
  return (
    <SimpleTooltip content={label}>
      <Button
        variant="ghost"
        size="icon"
        forceMinSize={false}
        aria-label={label}
        onMouseDown={(event) => event.preventDefault()}
        onClick={onClick}
      >
        {children}
      </Button>
    </SimpleTooltip>
  );
}
