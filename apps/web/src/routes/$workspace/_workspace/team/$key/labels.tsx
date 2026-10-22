import { createFileRoute, redirect } from "@tanstack/react-router";

export const Route = createFileRoute("/$workspace/_workspace/team/$key/labels")({
  beforeLoad: ({ params }) => {
    throw redirect({
      to: "/$workspace/settings/labels",
      params: { workspace: params.workspace },
      search: true,
      replace: true,
    });
  },
});
