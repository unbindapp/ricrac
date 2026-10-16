import { createFileRoute, redirect } from "@tanstack/react-router";

import { hasAssignedWorkQuery } from "@/lib/queries/issues";
import { teamsQuery } from "@/lib/queries/teams";

/** Opens on My issues when you have work assigned, else on your first team's issues. */
export const Route = createFileRoute("/$workspace/_workspace/")({
  beforeLoad: async ({ context: { queryClient, workspaceSummary }, params }) => {
    const [teams, hasWork] = await Promise.all([
      queryClient.ensureQueryData(teamsQuery(workspaceSummary.id)),
      queryClient.fetchQuery(hasAssignedWorkQuery(workspaceSummary.id)).catch(() => false),
    ]);
    const team = teams.find((item) => item.isMember) ?? teams[0];
    if (hasWork || !team) throw redirect({ to: "/$workspace/my-issues", params });
    throw redirect({ to: "/$workspace/team/$key/issues", params: { ...params, key: team.key } });
  },
});
