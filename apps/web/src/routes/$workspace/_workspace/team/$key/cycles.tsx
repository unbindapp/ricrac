import { createFileRoute, redirect } from "@tanstack/react-router";

export const Route = createFileRoute("/$workspace/_workspace/team/$key/cycles")({
  beforeLoad: ({ params }) => {
    throw redirect({
      to: "/$workspace/cycles",
      params: { workspace: params.workspace },
      search: true,
      replace: true,
    });
  },
});
