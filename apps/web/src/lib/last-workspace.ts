import { useEffect } from "react";

import type { WorkspaceSummary } from "@/api/account.gen";

const key = "ricrac:last-workspace";

function lastWorkspaceId(): string | null {
  try {
    return localStorage.getItem(key);
  } catch {
    return null;
  }
}

/** The workspace opened last in this browser, or the first one. */
export function landingWorkspace(workspaces: WorkspaceSummary[]) {
  const last = lastWorkspaceId();
  return workspaces.find((workspace) => workspace.id === last) ?? workspaces[0];
}

export function useRememberWorkspace(workspaceId: string) {
  useEffect(() => {
    try {
      localStorage.setItem(key, workspaceId);
    } catch {
      return;
    }
  }, [workspaceId]);
}
