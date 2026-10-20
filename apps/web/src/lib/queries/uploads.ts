import { useMutation } from "@tanstack/react-query";
import { useState } from "react";

import type { PreparedUpload } from "@/api/ops.gen";
import { ApiError, call } from "@/lib/server/client";

export const imageUploadAccept = "image/png,image/jpeg,image/gif,image/webp,image/svg+xml";

/** Called with how much of the file is sent, from 0 to 1. */
export type TUploadProgress = (fraction: number) => void;

/** Uploads a file: prepare_file_upload, then a PUT of its bytes to the one-time URL. */
export async function uploadFile(
  workspaceId: string,
  file: File,
  onProgress?: TUploadProgress,
): Promise<PreparedUpload> {
  const contentType = file.type || "application/octet-stream";
  const prepared = await call(workspaceId, "prepare_file_upload", {
    name: file.name,
    contentType,
    size: file.size,
  });
  await putFile(new URL(prepared.uploadUrl).pathname, file, contentType, onProgress);
  return prepared;
}

function putFile(path: string, file: File, contentType: string, onProgress?: TUploadProgress) {
  return new Promise<void>((resolve, reject) => {
    const request = new XMLHttpRequest();
    request.open("PUT", path);
    request.setRequestHeader("content-type", contentType);
    request.upload.onprogress = (event) => {
      if (event.lengthComputable) onProgress?.(event.loaded / event.total);
    };
    request.onerror = () =>
      reject(new ApiError(0, "Couldn't reach Ricrac. Try again in a moment."));
    request.onload = () => {
      if (request.status >= 200 && request.status < 300) {
        resolve();
        return;
      }
      reject(new ApiError(request.status, errorOf(request.responseText) ?? request.statusText));
    };
    request.send(file);
  });
}

function errorOf(body: string) {
  try {
    const data = JSON.parse(body);
    return typeof data?.error === "string" ? data.error : undefined;
  } catch {
    return undefined;
  }
}

export function useUploadFileMutation(workspaceId: string) {
  return useMutation({
    mutationFn: (file: File) => uploadFile(workspaceId, file),
    meta: { errorTitle: "Upload failed" },
  });
}

/** Uploads a file and gives the URL to show, embed or link. */
export function useUploadAsset(workspaceId: string) {
  return async (file: File, onProgress?: TUploadProgress) =>
    (await uploadFile(workspaceId, file, onProgress)).assetUrl;
}

/** Uploads for a Markdown editor, and whether one is still going (its embed isn't in the text yet). */
export function useEditorUploads(workspaceId: string) {
  const onUpload = useUploadAsset(workspaceId);
  const [isUploading, setIsUploading] = useState(false);
  return { editorProps: { onUpload, onUploadingChange: setIsUploading }, isUploading };
}
