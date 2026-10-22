import { createFileRoute, redirect } from "@tanstack/react-router";

const movedTeamPages = {
  workflow: "/$workspace/settings/workflow",
  labels: "/$workspace/settings/labels",
  templates: "/$workspace/settings/templates",
  cycles: "/$workspace/settings/cycles",
  triage: "/$workspace/settings/triage",
  estimates: "/$workspace/settings/estimates",
  github: "/$workspace/settings/github",
  "auto-archive": "/$workspace/settings/auto-archive",
  members: "/$workspace/settings/members",
} as const;

function isMovedTeamPage(page: string): page is keyof typeof movedTeamPages {
  return Object.hasOwn(movedTeamPages, page);
}

export const Route = createFileRoute("/$workspace/_workspace/settings/team/$")({
  beforeLoad: ({ params }) => {
    const page = params._splat?.split("/")[1] ?? "";
    throw redirect({
      to: isMovedTeamPage(page) ? movedTeamPages[page] : "/$workspace/settings",
      params: { workspace: params.workspace },
      replace: true,
    });
  },
});
