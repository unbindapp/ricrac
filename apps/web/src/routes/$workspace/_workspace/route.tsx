import { useQuery, useSuspenseQuery } from "@tanstack/react-query";
import { createFileRoute, Outlet, redirect } from "@tanstack/react-router";

import type { Me } from "@/api/account.gen";
import { TeamsProvider } from "@/components/workspace/teams-provider";
import { WorkspaceProvider } from "@/components/workspace/workspace-provider";
import { WorkspaceShell } from "@/components/workspace/workspace-shell";
import { landingWorkspace, useRememberWorkspace } from "@/lib/last-workspace";
import { meQuery } from "@/lib/queries/me";
import { workspaceQuery, workspaceRole } from "@/lib/queries/workspaces";

function findWorkspace(me: Me | null, urlKey: string) {
  return me?.workspaces.find((workspace) => workspace.urlKey === urlKey.toLowerCase());
}

export const Route = createFileRoute("/$workspace/_workspace")({
  beforeLoad: async ({ context: { queryClient }, params }) => {
    const cached = await queryClient.ensureQueryData(meQuery);
    const me = findWorkspace(cached, params.workspace)
      ? cached
      : await queryClient.fetchQuery(meQuery);
    if (!me) throw redirect({ to: "/sign-in" });
    const summary = findWorkspace(me, params.workspace);
    if (summary) return { workspaceSummary: summary };
    const fallback = landingWorkspace(me.workspaces);
    if (!fallback) throw redirect({ to: "/create-workspace" });
    throw redirect({ to: "/$workspace", params: { workspace: fallback.urlKey } });
  },
  loader: ({ context: { queryClient, workspaceSummary } }) =>
    queryClient.ensureQueryData(workspaceQuery(workspaceSummary.id)),
  component: WorkspaceLayout,
});

function WorkspaceLayout() {
  const { workspaceSummary } = Route.useRouteContext();
  const { data: me } = useQuery(meQuery);
  const summary = me?.workspaces.find((w) => w.id === workspaceSummary.id) ?? workspaceSummary;
  const { data: workspace } = useSuspenseQuery(workspaceQuery(summary.id));
  useRememberWorkspace(workspace.id);

  return (
    <WorkspaceProvider workspace={workspace} role={workspaceRole(summary)}>
      <TeamsProvider>
        <WorkspaceShell>
          <Outlet />
        </WorkspaceShell>
      </TeamsProvider>
    </WorkspaceProvider>
  );
}
