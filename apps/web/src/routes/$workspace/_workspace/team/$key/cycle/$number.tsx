import { createFileRoute, redirect } from "@tanstack/react-router";

export const Route = createFileRoute("/$workspace/_workspace/team/$key/cycle/$number")({
  beforeLoad: ({ params }) => {
    throw redirect({
      to: "/$workspace/cycle/$number",
      params: { workspace: params.workspace, number: params.number },
      search: true,
      replace: true,
    });
  },
});
