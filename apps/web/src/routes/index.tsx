import { createFileRoute, redirect } from "@tanstack/react-router";

import { LandingPage } from "@/components/landing/landing-page";
import { landingWorkspace } from "@/lib/last-workspace";
import { meQuery } from "@/lib/queries/me";

export const Route = createFileRoute("/")({
  beforeLoad: async ({ context }) => {
    const me = await context.queryClient.ensureQueryData(meQuery);
    if (!me) return;
    const workspace = landingWorkspace(me.workspaces);
    if (!workspace) throw redirect({ to: "/create-workspace" });
    throw redirect({ to: "/$workspace", params: { workspace: workspace.urlKey } });
  },
  component: LandingPage,
});
