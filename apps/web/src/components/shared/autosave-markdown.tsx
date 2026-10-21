import { useWorkspaceUsers } from "@/components/issues/picker-sources";
import { MarkdownEditor, type TEditorAddons } from "@/components/shared/markdown-editor";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { useUploadAsset } from "@/lib/queries/uploads";

type TProps = {
  /** The server's text; it shows others' changes as they come in. */
  value: string;
  label: string;
  placeholder: string;
  readOnly: boolean;
  /** Saves the edit from `base` and resolves to the server's text after it. */
  onSave: (markdown: string, base: string) => Promise<string>;
  addons?: TEditorAddons;
};

/** A description or document body that saves as you type, a moment after you stop. */
export function AutosaveMarkdown({ value, label, placeholder, readOnly, onSave, addons }: TProps) {
  const { workspace } = useWorkspace();
  const { users } = useWorkspaceUsers();
  const upload = useUploadAsset(workspace.id);
  if (readOnly && !value.trim()) return null;
  return (
    <MarkdownEditor
      label={label}
      value={value}
      placeholder={placeholder}
      readOnly={readOnly}
      users={users}
      onUpload={upload}
      autosave={{ onSave }}
      addons={addons}
    />
  );
}
