import { createContext, ReactNode, useContext, useMemo } from "react";

import type { Workspace } from "@/api/ops.gen";
import { canEditRole, isAdminRole, type TWorkspaceRole } from "@/lib/queries/workspaces";

type TWorkspaceContext = {
  /** Pass `workspace.id` to `call` and query keys, `workspace.urlKey` to route params. */
  workspace: Workspace;
  role: TWorkspaceRole;
  /** Owners, admins and members: everyone but guests. */
  canEdit: boolean;
  /** Owners and admins. */
  isAdmin: boolean;
};

const WorkspaceContext = createContext<TWorkspaceContext | null>(null);

export function WorkspaceProvider({
  workspace,
  role,
  children,
}: {
  workspace: Workspace;
  role: TWorkspaceRole;
  children: ReactNode;
}) {
  const value = useMemo<TWorkspaceContext>(
    () => ({ workspace, role, canEdit: canEditRole(role), isAdmin: isAdminRole(role) }),
    [workspace, role],
  );
  return <WorkspaceContext.Provider value={value}>{children}</WorkspaceContext.Provider>;
}

export function useWorkspace() {
  const context = useContext(WorkspaceContext);
  if (!context) throw new Error("useWorkspace must be used within a WorkspaceProvider");
  return context;
}
