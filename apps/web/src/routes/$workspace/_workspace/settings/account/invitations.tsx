import { useQuery } from "@tanstack/react-query";
import { createFileRoute, useNavigate } from "@tanstack/react-router";

import { MailIcon, UserPlusIcon } from "@/components/icons";
import { ListStates } from "@/components/layout/list-states";
import { SettingsPage } from "@/components/layout/settings-page";
import { SettingsSection } from "@/components/layout/settings-section";
import { useInviteStore } from "@/components/settings/invite-store";
import { Button } from "@/components/ui/button";
import { PendingInvitations } from "@/components/workspace/pending-invitations";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { meQuery } from "@/lib/queries/me";

export const Route = createFileRoute("/$workspace/_workspace/settings/account/invitations")({
  component: AccountInvitationsSettingsPage,
});

function AccountInvitationsSettingsPage() {
  const { canEdit } = useWorkspace();
  const openInvite = useInviteStore((state) => state.setOpen);
  const navigate = useNavigate();
  const { data: me, isPending, error, refetch } = useQuery(meQuery);
  const invitations = me?.invitations ?? [];
  return (
    <SettingsPage title="Invitations" description="Invite people, and join where you're invited.">
      {canEdit && (
        <SettingsSection
          title="Invite people"
          description="They join when they sign in with the invited email."
          actions={
            <Button size="sm" onClick={() => openInvite(true)}>
              <UserPlusIcon className="size-4" />
              Invite people
            </Button>
          }
        />
      )}
      <SettingsSection
        title="Invitations for you"
        description={me && `Invitations to other workspaces sent to ${me.email}.`}
      >
        <ListStates
          isPending={isPending}
          error={error}
          onRetry={refetch}
          isEmpty={invitations.length === 0}
          skeletonRows={2}
          empty={{ icon: MailIcon, title: "No invitations for you yet" }}
        >
          <PendingInvitations
            invitations={invitations}
            onAccepted={(workspace) =>
              void navigate({ to: "/$workspace", params: { workspace: workspace.urlKey } })
            }
          />
        </ListStates>
      </SettingsSection>
    </SettingsPage>
  );
}
