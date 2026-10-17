import { useNavigate } from "@tanstack/react-router";

import type { Issue, LabelRef } from "@/api/ops.gen";
import {
  patchToChanges,
  patchToOptimistic,
  type TIssuePatch,
} from "@/components/issues/issue-draft";
import { toast } from "@/components/ui/toast";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { useCopyToClipboard } from "@/lib/hooks/use-copy";
import {
  type TIssueLifecycleAction,
  useIssueLifecycleMutation,
  useUpdateIssueMutation,
} from "@/lib/queries/issues";

/** Changes properties of an issue, showing them at once. */
export function useIssuePatch() {
  const { workspace } = useWorkspace();
  const { mutate } = useUpdateIssueMutation(workspace.id);
  return (issue: { id: string; labels?: LabelRef[] }, patch: TIssuePatch) =>
    mutate({
      id: issue.id,
      changes: patchToChanges(patch, issue.labels),
      optimistic: patchToOptimistic(patch),
    });
}

export function useCopyIssue() {
  const { copyToClipboard } = useCopyToClipboard();
  const copy = (text: string, title: string) => {
    copyToClipboard(text);
    toast.add({ type: "success", title, description: text });
  };
  return {
    copyLink: (issue: Pick<Issue, "url">) => copy(issue.url, "Link copied"),
    copyIdentifier: (issue: Pick<Issue, "identifier">) => copy(issue.identifier, "ID copied"),
    copyBranch: (issue: Pick<Issue, "branchName">) => copy(issue.branchName, "Branch name copied"),
  };
}

const lifecycleToasts: Record<TIssueLifecycleAction, [string, string]> = {
  archive: ["Issue archived", "issues archived"],
  unarchive: ["Issue unarchived", "issues unarchived"],
  delete: ["Issue moved to the trash", "issues moved to the trash"],
  restore: ["Issue restored", "issues restored"],
};

export function useIssueLifecycle() {
  const { workspace } = useWorkspace();
  const { mutateAsync, isPending } = useIssueLifecycleMutation(workspace.id);
  const run = async (ids: string[], action: TIssueLifecycleAction) => {
    await mutateAsync({ ids, action });
    const [one, many] = lifecycleToasts[action];
    toast.add({ type: "success", title: ids.length === 1 ? one : `${ids.length} ${many}` });
  };
  return { run, isPending };
}

export function useOpenIssuePage() {
  const navigate = useNavigate();
  const { workspace } = useWorkspace();
  return (identifier: string) =>
    navigate({
      to: "/$workspace/issue/$identifier",
      params: { workspace: workspace.urlKey, identifier },
    });
}
