import { useQuery, useSuspenseQuery } from "@tanstack/react-query";
import { createFileRoute, Outlet, redirect } from "@tanstack/react-router";
import { zodValidator } from "@tanstack/zod-adapter";
import { z } from "zod";

import type { Me } from "@/api/account.gen";
import { TeamsProvider } from "@/components/workspace/teams-provider";
import { VisitorShell } from "@/components/workspace/visitor-shell";
import { WorkspaceProvider } from "@/components/workspace/workspace-provider";
import { WorkspaceShell } from "@/components/workspace/workspace-shell";
import { landingWorkspace, useRememberWorkspace } from "@/lib/last-workspace";
import { UiThemeProvider } from "@/themes/ui-theme-provider";
import { meQuery } from "@/lib/queries/me";
import { fetchPublicWorkspace, workspaceQuery, workspaceRole } from "@/lib/queries/workspaces";

function findWorkspace(me: Me | null, urlKey: string) {
  return me?.workspaces.find((workspace) => workspace.urlKey === urlKey.toLowerCase());
}

/** The pages visitors of a public workspace may open: /issues and /issue/ENG-123. */
const VISITOR_PAGES = ["issues", "issue"];

function isVisitorPage(pathname: string) {
  return VISITOR_PAGES.includes(pathname.split("/")[2] ?? "");
}

const searchSchema = z.object({
  /** The settings page open in a window over this one, like /settings/members. */
  settings: z.string().optional(),
});

export const Route = createFileRoute("/$workspace/_workspace")({
  validateSearch: zodValidator(searchSchema),
  beforeLoad: async ({ context: { queryClient }, params, location, search }) => {
    const cached = await queryClient.ensureQueryData(meQuery);
    const me = findWorkspace(cached, params.workspace)
      ? cached
      : await queryClient.fetchQuery(meQuery);
    const summary = findWorkspace(me, params.workspace);
    if (summary) return { workspaceSummary: summary, isVisitor: false };
    const visited = await fetchPublicWorkspace(queryClient, params.workspace);
    if (visited && (!isVisitorPage(location.pathname) || search.settings)) {
      throw redirect({ to: "/$workspace/issues", params });
    }
    if (visited) return { workspaceSummary: visited, isVisitor: true };
    if (!me) throw redirect({ to: "/sign-in", search: { redirect: location.href } });
    const fallback = landingWorkspace(me.workspaces);
    if (!fallback) throw redirect({ to: "/create-workspace" });
    throw redirect({ to: "/$workspace", params: { workspace: fallback.urlKey } });
  },
  loader: ({ context: { queryClient, workspaceSummary } }) =>
    queryClient.ensureQueryData(workspaceQuery(workspaceSummary.id)),
  component: WorkspaceLayout,
});

function WorkspaceLayout() {
  const { workspaceSummary, isVisitor } = Route.useRouteContext();
  const { data: me } = useQuery(meQuery);
  const summary = me?.workspaces.find((w) => w.id === workspaceSummary.id) ?? workspaceSummary;
  const { data: workspace } = useSuspenseQuery(workspaceQuery(summary.id));
  useRememberWorkspace(workspace.id);
  const Shell = isVisitor ? VisitorShell : WorkspaceShell;

  return (
    <WorkspaceProvider workspace={workspace} role={workspaceRole(summary)} isVisitor={isVisitor}>
      <UiThemeProvider themeId={workspace.theme} workspaceUrlKey={workspace.urlKey}>
        <TeamsProvider>
          <Shell>
            <Outlet />
          </Shell>
        </TeamsProvider>
      </UiThemeProvider>
    </WorkspaceProvider>
  );
}
