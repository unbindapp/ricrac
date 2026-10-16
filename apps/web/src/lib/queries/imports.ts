import { useMutation, useQueryClient } from "@tanstack/react-query";

import type { ImportIssuesArgs } from "@/api/ops.gen";
import { call } from "@/lib/server/client";

export type TImportIssue = ImportIssuesArgs["issues"] extends (infer T)[] | undefined ? T : never;

/** Bodies above this go up as a file (prepare_file_upload) instead: requests stop at 8 MB. */
const INLINE_LIMIT_BYTES = 6 * 1024 * 1024;

async function uploadIssuesFile(workspaceId: string, issues: TImportIssue[]) {
  const body = new Blob([JSON.stringify({ issues })], { type: "application/json" });
  const upload = await call(workspaceId, "prepare_file_upload", {
    name: "issues.json",
    contentType: "application/json",
    size: body.size,
  });
  const response = await fetch(new URL(upload.uploadUrl).pathname, { method: "PUT", body });
  if (!response.ok) throw new Error("Could not upload the issues");
  return upload.fileId;
}

/** Imports issues into a team; dryRun only checks them and reports. */
export async function importIssues(
  workspaceId: string,
  { team, issues, dryRun }: { team: string; issues: TImportIssue[]; dryRun: boolean },
) {
  const size = new Blob([JSON.stringify(issues)]).size;
  if (size <= INLINE_LIMIT_BYTES) {
    return call(workspaceId, "import_issues", { team, issues, dryRun });
  }
  const fileId = await uploadIssuesFile(workspaceId, issues);
  return call(workspaceId, "import_issues", { team, fileId, dryRun });
}

export function useImportIssuesMutation(workspaceId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (input: { team: string; issues: TImportIssue[]; dryRun: boolean }) =>
      importIssues(workspaceId, input),
    meta: { silent: true },
    onSuccess: (result) => {
      if (result.dryRun) return;
      return queryClient.invalidateQueries({ queryKey: [workspaceId] });
    },
  });
}
