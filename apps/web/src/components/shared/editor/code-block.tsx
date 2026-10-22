import { CodeBlockLowlight } from "@tiptap/extension-code-block-lowlight";
import {
  NodeViewContent,
  NodeViewWrapper,
  ReactNodeViewRenderer,
  type ReactNodeViewProps,
} from "@tiptap/react";
import { common, createLowlight } from "lowlight";
import { ChevronDownIcon } from "lucide-react";

import { Button } from "@/components/ui/button";
import DropdownSelect from "@/components/ui/dropdown-select";

const lowlight = createLowlight(common);
const PLAIN = "plaintext";

const languages = [
  { value: PLAIN, label: "Plain text" },
  ...lowlight
    .listLanguages()
    .filter((language) => language !== PLAIN)
    .map((language) => ({ value: language, label: language }))
    .sort((a, b) => a.label.localeCompare(b.label)),
];

export const EditorCodeBlock = CodeBlockLowlight.extend({
  addNodeView() {
    return ReactNodeViewRenderer(CodeBlockView);
  },
}).configure({ lowlight });

function CodeBlockView({ node, editor, updateAttributes }: ReactNodeViewProps) {
  const language = (node.attrs.language as string | null) || PLAIN;
  const label = languages.find((option) => option.value === language)?.label ?? language;
  return (
    <NodeViewWrapper className="group/code relative">
      {editor.isEditable && (
        <div contentEditable={false} className="absolute top-1.5 right-1.5 z-10">
          <DropdownSelect
            items={languages}
            value={language}
            align="end"
            classNameContent="w-48 max-h-72"
            onChange={(value) => updateAttributes({ language: value === PLAIN ? null : value })}
          >
            {() => (
              <Button
                variant="ghost"
                size="sm"
                forceMinSize={false}
                aria-label="Code language"
                className="text-muted-foreground h-6 px-1.5 font-mono text-xs font-normal opacity-0 group-hover/code:opacity-100 focus-visible:opacity-100 data-popup-open:opacity-100"
              >
                {label}
                <ChevronDownIcon className="size-3.5" />
              </Button>
            )}
          </DropdownSelect>
        </div>
      )}
      <pre>
        <NodeViewContent<"code"> as="code" />
      </pre>
    </NodeViewWrapper>
  );
}
