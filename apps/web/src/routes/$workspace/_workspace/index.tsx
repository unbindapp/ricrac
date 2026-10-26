import { createFileRoute, redirect } from "@tanstack/react-router";

import { hasAssignedWorkQuery } from "@/lib/queries/issues";

/** Opens on the inbox in Dank; elsewhere on My issues when you have work assigned, else on the workspace's issues. */
export const Route = createFileRoute("/$workspace/_workspace/")({
  beforeLoad: async ({ context: { queryClient, workspaceSummary }, params }) => {
    if (workspaceSummary.theme === "dank") throw redirect({ to: "/$workspace/inbox", params });
    const hasWork = await queryClient
      .fetchQuery(hasAssignedWorkQuery(workspaceSummary.id))
      .catch(() => false);
    if (hasWork) throw redirect({ to: "/$workspace/my-issues", params });
    throw redirect({ to: "/$workspace/issues", params });
  },
});
