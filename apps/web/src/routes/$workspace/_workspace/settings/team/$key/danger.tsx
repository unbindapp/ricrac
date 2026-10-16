import { createFileRoute, useNavigate } from "@tanstack/react-router";

import { SettingsPage } from "@/components/layout/settings-page";
import { SettingsSection } from "@/components/layout/settings-section";
import { ConfirmDialog } from "@/components/shared/confirm-dialog";
import { Button } from "@/components/ui/button";
import { useRequiredTeamFromRoute } from "@/components/workspace/use-team-from-route";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { useDeleteTeamMutation, useLeaveTeamMutation } from "@/lib/queries/teams";

export const Route = createFileRoute("/$workspace/_workspace/settings/team/$key/danger")({
  component: TeamDangerSettingsPage,
});

function TeamDangerSettingsPage() {
  const team = useRequiredTeamFromRoute();
  const { workspace } = useWorkspace();
  const navigate = useNavigate();
  const { mutateAsync: deleteTeam } = useDeleteTeamMutation(workspace.id);
  const { mutateAsync: leaveTeam } = useLeaveTeamMutation(workspace.id);
  const toWorkspace = () =>
    navigate({ to: "/$workspace", params: { workspace: workspace.urlKey } });
  return (
    <SettingsPage title="Danger zone" description="Leave or delete this team.">
      {team.isMember && (
        <SettingsSection
          variant="destructive"
          title="Leave team"
          description="It leaves your sidebar. The last member of a private team can't leave it."
          footer={
            <ConfirmDialog
              trigger={
                <Button variant="outline" size="sm">
                  Leave team
                </Button>
              }
              title={`Leave ${team.name}`}
              description={
                team.private ? "You lose access to this private team and its issues." : undefined
              }
              confirmLabel="Leave team"
              onConfirm={async () => {
                await leaveTeam(team.id);
                await toWorkspace();
              }}
            />
          }
        />
      )}
      <SettingsSection
        variant="destructive"
        title="Delete team"
        description="Workspace admins and team owners can delete a team. It's archived: it leaves every list and its key becomes free, while its issues stay readable."
        footer={
          <ConfirmDialog
            trigger={
              <Button variant="destructive" size="sm">
                Delete team
              </Button>
            }
            title={`Delete ${team.name}`}
            description="The team leaves every list and its key becomes free."
            confirmLabel="Delete team"
            confirmText={team.key}
            onConfirm={async () => {
              await deleteTeam(team.id);
              await toWorkspace();
            }}
          />
        }
      />
    </SettingsPage>
  );
}
