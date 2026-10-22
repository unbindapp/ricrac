import { createFileRoute, redirect } from "@tanstack/react-router";

export const Route = createFileRoute("/$workspace/_workspace/settings_/teams")({
  beforeLoad: ({ params }) => {
    throw redirect({ to: "/$workspace/settings", params, replace: true });
  },
});
