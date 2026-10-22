import { useSuspenseQuery } from "@tanstack/react-query";

import { useWorkspace } from "@/components/workspace/workspace-provider";
import { teamQuery, teamsQuery } from "@/lib/queries/teams";

/** The workspace's team, which holds its issue settings; suspends while it loads. */
export function useSettingsTeam() {
  const { workspace } = useWorkspace();
  const { data: teams } = useSuspenseQuery(teamsQuery(workspace.id));
  const team = teams.find((item) => !item.archivedAt) ?? teams[0];
  if (!team) throw new Error("This workspace has no issue settings yet.");
  return team;
}

/** The workspace's team with its settings, members and statuses (get_team). */
export function useSettingsTeamDetail() {
  const team = useSettingsTeam();
  const { workspace } = useWorkspace();
  return useSuspenseQuery(teamQuery(workspace.id, team.id)).data;
}
