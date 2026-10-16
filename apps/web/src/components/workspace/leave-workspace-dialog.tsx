import { useNavigate } from "@tanstack/react-router";

import { ConfirmDialog } from "@/components/shared/confirm-dialog";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { useLeaveWorkspaceMutation } from "@/lib/queries/workspaces";

export function LeaveWorkspaceDialog({
  isOpen,
  onOpenChange,
}: {
  isOpen: boolean;
  onOpenChange: (isOpen: boolean) => void;
}) {
  const { workspace } = useWorkspace();
  const navigate = useNavigate();
  const { mutateAsync: leave } = useLeaveWorkspaceMutation(workspace.id);
  return (
    <ConfirmDialog
      open={isOpen}
      onOpenChange={onOpenChange}
      title={`Leave ${workspace.name}`}
      description="You lose access to its teams and issues until someone invites you again."
      confirmLabel="Leave workspace"
      onConfirm={async () => {
        await leave();
        await navigate({ to: "/" });
      }}
    />
  );
}
