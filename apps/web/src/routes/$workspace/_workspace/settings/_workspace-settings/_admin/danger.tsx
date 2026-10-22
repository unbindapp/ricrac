import { createFileRoute, useNavigate } from "@tanstack/react-router";
import { useState } from "react";

import { SettingsPage } from "@/components/layout/settings-page";
import { SettingsSection } from "@/components/layout/settings-section";
import { ConfirmDialog } from "@/components/shared/confirm-dialog";
import { Button } from "@/components/ui/button";
import { LeaveWorkspaceDialog } from "@/components/workspace/leave-workspace-dialog";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { useDeleteWorkspaceMutation } from "@/lib/queries/workspaces";

export const Route = createFileRoute(
  "/$workspace/_workspace/settings/_workspace-settings/_admin/danger",
)({
  component: WorkspaceDangerSettingsPage,
});

function WorkspaceDangerSettingsPage() {
  const { workspace, role } = useWorkspace();
  const navigate = useNavigate();
  const [isLeaving, setIsLeaving] = useState(false);
  const { mutateAsync: deleteWorkspace } = useDeleteWorkspaceMutation(workspace.id);
  const isOwner = role === "owner";

  return (
    <SettingsPage title="Danger zone" description="Actions here cannot be undone.">
      <SettingsSection
        variant="destructive"
        title="Leave workspace"
        description="You lose access to its issues until someone invites you again. A workspace's only owner can't leave it."
        footer={
          <Button variant="outline" size="sm" onClick={() => setIsLeaving(true)}>
            Leave workspace
          </Button>
        }
      />
      <LeaveWorkspaceDialog isOpen={isLeaving} onOpenChange={setIsLeaving} />
      <SettingsSection
        variant="destructive"
        title="Delete workspace"
        description={
          isOwner
            ? "Deletes the workspace with every issue, project and document in it, for every member."
            : "Only owners can delete the workspace."
        }
        footer={
          <ConfirmDialog
            trigger={
              <Button variant="destructive" size="sm" disabled={!isOwner}>
                Delete workspace
              </Button>
            }
            title={`Delete ${workspace.name}`}
            description="Everything in this workspace is deleted permanently for every member."
            confirmLabel="Delete workspace"
            confirmText={workspace.urlKey}
            onConfirm={async () => {
              await deleteWorkspace(workspace.urlKey);
              await navigate({ to: "/" });
            }}
          />
        }
      />
    </SettingsPage>
  );
}
