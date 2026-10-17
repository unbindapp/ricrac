import { useMutation } from "@tanstack/react-query";

import type { PreparedUpload } from "@/api/ops.gen";
import { ApiError, call } from "@/lib/server/client";

export const imageUploadAccept = "image/png,image/jpeg,image/gif,image/webp,image/svg+xml";

/** Uploads a file: prepare_file_upload, then a PUT of its bytes to the one-time URL. */
export async function uploadFile(workspaceId: string, file: File): Promise<PreparedUpload> {
  const contentType = file.type || "application/octet-stream";
  const prepared = await call(workspaceId, "prepare_file_upload", {
    name: file.name,
    contentType,
    size: file.size,
  });
  const response = await fetch(new URL(prepared.uploadUrl).pathname, {
    method: "PUT",
    headers: { "content-type": contentType },
    body: file,
  }).catch(() => {
    throw new ApiError(0, "Couldn't reach Ricrac. Try again in a moment.");
  });
  if (response.ok) return prepared;
  const data = await response.json().catch(() => undefined);
  throw new ApiError(
    response.status,
    typeof data?.error === "string" ? data.error : response.statusText,
  );
}

export function useUploadFileMutation(workspaceId: string) {
  return useMutation({
    mutationFn: (file: File) => uploadFile(workspaceId, file),
    meta: { errorTitle: "Upload failed" },
  });
}

/** For editors: uploads an image and gives the URL to embed. */
export function useUploadImage(workspaceId: string) {
  return async (file: File) => (await uploadFile(workspaceId, file)).assetUrl;
}
