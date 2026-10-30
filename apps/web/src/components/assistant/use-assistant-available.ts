import { useQuery } from "@tanstack/react-query";

import { useWorkspace } from "@/components/workspace/workspace-provider";
import { meQuery } from "@/lib/queries/me";

function useWorkspaceSummary() {
  const { workspace } = useWorkspace();
  const { data: me } = useQuery(meQuery);
  return me?.workspaces.find((each) => each.id === workspace.id);
}

/** The server turned Claude on for this person here, and they can change things, so not guests or visitors. */
export function useIsAssistantAvailable() {
  const { canEdit } = useWorkspace();
  const summary = useWorkspaceSummary();
  return canEdit && Boolean(summary?.assistant);
}

/** This person may turn Claude on here and pick who asks it. */
export function useManagesAssistant() {
  return Boolean(useWorkspaceSummary()?.managesAssistant);
}
