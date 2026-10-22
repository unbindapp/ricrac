import { createFileRoute, Outlet, useNavigate } from "@tanstack/react-router";

import { SettingsWindow } from "@/components/layout/settings-window";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { useSettingsReturnHref } from "@/lib/stores/settings-return";

export const Route = createFileRoute("/$workspace/_workspace/settings")({
  component: SettingsRoute,
});

function SettingsRoute() {
  const { workspace } = useWorkspace();
  const returnHref = useSettingsReturnHref(workspace.urlKey);
  const navigate = useNavigate();

  const close = () => {
    if (returnHref) {
      void navigate({ href: returnHref });
      return;
    }
    void navigate({ to: "/$workspace", params: { workspace: workspace.urlKey } });
  };

  return (
    <SettingsWindow onClose={close}>
      <Outlet />
    </SettingsWindow>
  );
}
