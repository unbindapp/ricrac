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
  /** Someone who isn't a member reading a public workspace: its issues only, read-only. */
  isVisitor: boolean;
};

const WorkspaceContext = createContext<TWorkspaceContext | null>(null);

export function WorkspaceProvider({
  workspace,
  role,
  isVisitor,
  children,
}: {
  workspace: Workspace;
  role: TWorkspaceRole;
  isVisitor: boolean;
  children: ReactNode;
}) {
  const value = useMemo<TWorkspaceContext>(
    () => ({
      workspace,
      role,
      canEdit: !isVisitor && canEditRole(role),
      isAdmin: !isVisitor && isAdminRole(role),
      isVisitor,
    }),
    [workspace, role, isVisitor],
  );
  return <WorkspaceContext.Provider value={value}>{children}</WorkspaceContext.Provider>;
}

export function useWorkspace() {
  const context = useContext(WorkspaceContext);
  if (!context) throw new Error("useWorkspace must be used within a WorkspaceProvider");
  return context;
}
