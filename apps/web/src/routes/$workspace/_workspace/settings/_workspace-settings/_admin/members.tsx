import { useQuery } from "@tanstack/react-query";
import { createFileRoute } from "@tanstack/react-router";
import type { ReactNode } from "react";
import { MailIcon, Trash2Icon, UserPlusIcon, UsersIcon } from "lucide-react";

import type { Invitation, WorkspaceUser } from "@/api/ops.gen";
import { List, ListRow, ListRowText } from "@/components/layout/list";
import { ListStates } from "@/components/layout/list-states";
import { SettingsPage } from "@/components/layout/settings-page";
import { SettingsSection } from "@/components/layout/settings-section";
import { useInviteStore } from "@/components/settings/invite-store";
import { Avatar } from "@/components/shared/avatar";
import { ConfirmDialog } from "@/components/shared/confirm-dialog";
import { RelativeTime } from "@/components/shared/relative-time";
import { RoleSelect } from "@/components/shared/role-select";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { useRemoveUserMutation, useUpdateUserMutation, usersQuery } from "@/lib/queries/users";
import {
  invitationsQuery,
  type TWorkspaceRole,
  useDeleteInvitationMutation,
  WORKSPACE_ROLES,
  workspaceRoleLabels,
} from "@/lib/queries/workspaces";

export const Route = createFileRoute(
  "/$workspace/_workspace/settings/_workspace-settings/_admin/members",
)({
  component: WorkspaceMembersSettingsPage,
});

function WorkspaceMembersSettingsPage() {
  const openInvite = useInviteStore((state) => state.setOpen);
  const inviteButton = (
    <Button size="sm" onClick={() => openInvite(true)}>
      <UserPlusIcon className="size-4" />
      Invite people
    </Button>
  );
  return (
    <SettingsPage title="Members" description="People in this workspace and their roles.">
      <MembersSection actions={inviteButton} />
      <InvitationsSection />
    </SettingsPage>
  );
}

function MembersSection({ actions }: { actions: ReactNode }) {
  const { workspace } = useWorkspace();
  const { data: users = [], isPending, error, refetch } = useQuery(usersQuery(workspace.id));
  return (
    <SettingsSection
      title="Members"
      description={peopleCount(users.length || workspace.memberCount)}
      actions={actions}
    >
      <ListStates
        isPending={isPending}
        error={error}
        onRetry={refetch}
        isEmpty={users.length === 0}
        empty={{ icon: UsersIcon, title: "No members yet" }}
      >
        <List>
          {users.map((user) => (
            <MemberRow key={user.id} user={user} />
          ))}
        </List>
      </ListStates>
    </SettingsSection>
  );
}

function peopleCount(count: number) {
  return `${count} ${count === 1 ? "person" : "people"}`;
}

/** Admins hand out every role but owner; owner changes need an owner. */
function assignableRoles(myRole: TWorkspaceRole, theirRole: TWorkspaceRole) {
  if (myRole === "owner") return WORKSPACE_ROLES;
  if (theirRole === "owner") return [theirRole];
  return WORKSPACE_ROLES.filter((role) => role !== "owner");
}

function MemberRow({ user }: { user: WorkspaceUser }) {
  const { workspace, role: myRole } = useWorkspace();
  const { mutate: update, isPending: isUpdating } = useUpdateUserMutation(workspace.id);
  const { mutateAsync: remove } = useRemoveUserMutation(workspace.id);
  const roles = assignableRoles(myRole, user.role);
  const canRemove = !user.isMe && (myRole === "owner" || user.role !== "owner");

  return (
    <ListRow
      actions={
        <>
          <RoleSelect
            value={user.role}
            roles={roles}
            label={`Role for ${user.name}`}
            disabled={user.isMe || roles.length === 1 || isUpdating}
            onChange={(role) => update({ id: user.id, role })}
          />
          <ConfirmDialog
            trigger={
              <Button
                variant="ghost"
                size="icon"
                aria-label={`Remove ${user.name}`}
                disabled={!canRemove}
              >
                <Trash2Icon className="size-4" />
              </Button>
            }
            title={`Remove ${user.name}`}
            description="They lose access to this workspace. Their issues, comments and history stay."
            confirmLabel="Remove member"
            onConfirm={() => remove(user.id)}
          />
        </>
      }
    >
      <Avatar id={user.id} name={user.name} src={user.avatarUrl} size="sm" />
      <ListRowText primary={user.name} secondary={user.email} />
      {user.isMe && <Badge variant="muted">You</Badge>}
    </ListRow>
  );
}

function InvitationsSection() {
  const { workspace } = useWorkspace();
  const {
    data: invitations = [],
    isPending,
    error,
    refetch,
  } = useQuery(invitationsQuery(workspace.id));
  return (
    <SettingsSection
      title="Pending invitations"
      description="They join when they sign in with the invited email."
    >
      <ListStates
        isPending={isPending}
        error={error}
        onRetry={refetch}
        isEmpty={invitations.length === 0}
        skeletonRows={2}
        empty={{ icon: MailIcon, title: "No pending invitations yet" }}
      >
        <List>
          {invitations.map((invitation) => (
            <InvitationRow key={invitation.id} invitation={invitation} />
          ))}
        </List>
      </ListStates>
    </SettingsSection>
  );
}

function InvitationRow({ invitation }: { invitation: Invitation }) {
  const { workspace } = useWorkspace();
  const { mutateAsync: revoke } = useDeleteInvitationMutation(workspace.id);
  return (
    <ListRow
      actions={
        <>
          <span className="text-muted-foreground hidden text-xs sm:inline">
            Expires <RelativeTime value={invitation.expiresAt} />
          </span>
          <ConfirmDialog
            trigger={
              <Button
                variant="ghost"
                size="icon"
                aria-label={`Revoke invitation for ${invitation.email}`}
              >
                <Trash2Icon className="size-4" />
              </Button>
            }
            title="Revoke invitation"
            description={`${invitation.email} can no longer join with it.`}
            confirmLabel="Revoke invitation"
            onConfirm={() => revoke(invitation.id)}
          />
        </>
      }
    >
      <MailIcon className="text-muted-foreground size-4" />
      <ListRowText primary={invitation.email} secondary={workspaceRoleLabels[invitation.role]} />
    </ListRow>
  );
}
