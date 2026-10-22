import { createFileRoute, redirect } from "@tanstack/react-router";

export const Route = createFileRoute("/$workspace/_workspace/team/$key/triage")({
  beforeLoad: ({ params }) => {
    throw redirect({
      to: "/$workspace/triage",
      params: { workspace: params.workspace },
      search: true,
      replace: true,
    });
  },
});
