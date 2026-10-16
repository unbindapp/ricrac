import { useMutation } from "@tanstack/react-query";
import { useState } from "react";

import { useMentionUsers } from "@/components/projects/people";
import { MarkdownEditor } from "@/components/shared/markdown-editor";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { call } from "@/lib/server/client";

function readAsDataUrl(file: File) {
  return new Promise<string>((resolve, reject) => {
    const reader = new FileReader();
    reader.onload = () => resolve(String(reader.result));
    reader.onerror = () => reject(reader.error ?? new Error("Could not read the file"));
    reader.readAsDataURL(file);
  });
}

/** Uploads a pasted or dropped image and gives the link the editor embeds. */
export function useUploadImage() {
  const { workspace } = useWorkspace();
  const { mutateAsync } = useMutation({
    mutationFn: async (file: File) =>
      call(workspace.id, "upload_file", {
        base64: await readAsDataUrl(file),
        contentType: file.type,
        name: file.name,
      }),
    meta: { errorTitle: "Upload failed" },
  });
  return async (file: File) => (await mutateAsync(file)).assetUrl;
}

type TProps = {
  initialValue: string;
  label: string;
  placeholder: string;
  readOnly: boolean;
  onSave: (markdown: string) => Promise<unknown>;
};

/** A description or document body that saves itself while you write. */
export function MarkdownBody({ initialValue, label, placeholder, readOnly, onSave }: TProps) {
  const [value] = useState(initialValue);
  const users = useMentionUsers();
  const uploadImage = useUploadImage();
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
