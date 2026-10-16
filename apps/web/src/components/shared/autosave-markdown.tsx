import { useState } from "react";

import { useWorkspaceUsers } from "@/components/issues/picker-sources";
import { MarkdownEditor } from "@/components/shared/markdown-editor";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { useUploadImage } from "@/lib/queries/uploads";

type TProps = {
  initialValue: string;
  label: string;
  placeholder: string;
  readOnly: boolean;
  onSave: (markdown: string) => Promise<unknown>;
};

/** Saves as you type, a moment after you stop; text that didn't change isn't saved. */
export function AutosaveMarkdown({ initialValue, label, placeholder, readOnly, onSave }: TProps) {
  const { workspace } = useWorkspace();
  const [value] = useState(initialValue);
  const { users } = useWorkspaceUsers();
  const uploadImage = useUploadImage(workspace.id);
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
