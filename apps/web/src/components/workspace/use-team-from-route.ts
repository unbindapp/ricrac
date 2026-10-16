import { useSuspenseQuery } from "@tanstack/react-query";
import { useParams } from "@tanstack/react-router";

import { useTeams } from "@/components/workspace/teams-provider";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { teamQuery } from "@/lib/queries/teams";

export function useTeamFromRoute() {
  const { key } = useParams({ strict: false });
  const { teamByKey } = useTeams();
  return key ? teamByKey(key) : undefined;
}

export function useRequiredTeamFromRoute() {
  const team = useTeamFromRoute();
  if (!team)
    throw new Error("useRequiredTeamFromRoute must be used below a route that resolves the team");
  return team;
}

/** The route's team with its settings, members and statuses (get_team), by id so key renames keep it. */
export function useTeamDetailFromRoute() {
  const team = useRequiredTeamFromRoute();
  const { workspace } = useWorkspace();
  return useSuspenseQuery(teamQuery(workspace.id, team.id)).data;
}
