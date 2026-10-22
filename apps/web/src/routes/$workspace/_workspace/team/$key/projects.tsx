import { createFileRoute, redirect } from "@tanstack/react-router";

export const Route = createFileRoute("/$workspace/_workspace/team/$key/projects")({
  beforeLoad: ({ params }) => {
    throw redirect({
      to: "/$workspace/projects",
      params: { workspace: params.workspace },
      search: true,
      replace: true,
    });
  },
});
