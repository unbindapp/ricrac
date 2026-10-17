import { useWorkspaceUsers } from "@/components/issues/picker-sources";
import { MarkdownEditor } from "@/components/shared/markdown-editor";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { useUploadImage } from "@/lib/queries/uploads";

type TProps = {
  /** The server's text; it shows others' changes as they come in. */
  value: string;
  label: string;
  placeholder: string;
  readOnly: boolean;
  /** Saves the edit from `base` and resolves to the server's text after it. */
  onSave: (markdown: string, base: string) => Promise<string>;
};

/** A description or document body that saves as you type, a moment after you stop. */
export function AutosaveMarkdown({ value, label, placeholder, readOnly, onSave }: TProps) {
  const { workspace } = useWorkspace();
  const { users } = useWorkspaceUsers();
  const uploadImage = useUploadImage(workspace.id);
  if (readOnly && !value.trim()) return null;
  return (
    <MarkdownEditor
      label={label}
      value={value}
      placeholder={placeholder}
      readOnly={readOnly}
      users={users}
      onUploadImage={uploadImage}
      autosave={{ onSave }}
    />
  );
}
