import { useQuery } from "@tanstack/react-query";
import { createFileRoute } from "@tanstack/react-router";
import { Trash2Icon, UserPlusIcon, UsersIcon } from "lucide-react";
import { useState } from "react";

import type { TeamDetail, TeamMember } from "@/api/ops.gen";
import { EmptyState } from "@/components/layout/empty-state";
import { List, ListRow, ListRowText } from "@/components/layout/list";
import { SettingsPage } from "@/components/layout/settings-page";
import { SettingsSection } from "@/components/layout/settings-section";
import { Avatar } from "@/components/shared/avatar";
import { ConfirmDialog } from "@/components/shared/confirm-dialog";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { useTeamDetailFromRoute } from "@/components/workspace/use-team-from-route";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { useSaveTeamMutation } from "@/lib/queries/teams";
import { usersQuery } from "@/lib/queries/users";

export const Route = createFileRoute("/$workspace/_workspace/settings/team/$key/members")({
  component: TeamMembersSettingsPage,
});

function TeamMembersSettingsPage() {
  const team = useTeamDetailFromRoute();
  return (
    <SettingsPage title="Members" description="People who belong to this team.">
      <SettingsSection title="Add member">
        <AddMember team={team} />
      </SettingsSection>
      <SettingsSection title="Members">
        {team.members.length === 0 ? (
          <EmptyState icon={UsersIcon} title="No members yet" />
        ) : (
          <List>
            {team.members.map((member) => (
              <TeamMemberRow key={member.id} team={team} member={member} />
            ))}
          </List>
        )}
      </SettingsSection>
    </SettingsPage>
  );
}

function AddMember({ team }: { team: TeamDetail }) {
  const { workspace } = useWorkspace();
  const { data: users } = useQuery(usersQuery(workspace.id));
  const [userId, setUserId] = useState<string | null>(null);
  const { mutate: saveTeam, isPending } = useSaveTeamMutation(workspace.id);
  const memberIds = new Set(team.members.map((member) => member.id));
  const candidates = (users ?? []).filter((user) => !memberIds.has(user.id));
  const items = candidates.map((user) => ({ value: user.id, label: user.name }));

  if (users && candidates.length === 0) {
    return (
      <p className="text-muted-foreground text-sm">Everyone in the workspace is in this team.</p>
    );
  }

  return (
    <div className="flex items-center gap-2">
      <Select items={items} value={userId} onValueChange={(value) => setUserId(value)}>
        <SelectTrigger aria-label="Workspace member" className="min-w-0 flex-1">
          <SelectValue placeholder="Pick a workspace member" />
        </SelectTrigger>
        <SelectContent>
          {candidates.map((user) => (
            <SelectItem key={user.id} value={user.id}>
              <Avatar id={user.id} name={user.name} src={user.avatarUrl} size="xs" />
              <span className="truncate">{user.name}</span>
              <span className="text-muted-foreground truncate text-xs">{user.email}</span>
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
      <Button
        disabled={!userId}
        isPending={isPending}
        onClick={() =>
          userId &&
          saveTeam({ id: team.id, addMembers: [userId] }, { onSuccess: () => setUserId(null) })
        }
      >
        <UserPlusIcon className="size-4" />
        Add
      </Button>
    </div>
  );
}

function TeamMemberRow({ team, member }: { team: TeamDetail; member: TeamMember }) {
  const { workspace } = useWorkspace();
  const { mutateAsync: saveTeam } = useSaveTeamMutation(workspace.id);
  return (
    <ListRow
      actions={
        <ConfirmDialog
          trigger={
            <Button
              variant="ghost"
              size="icon"
              aria-label={`Remove ${member.name} from ${team.name}`}
            >
              <Trash2Icon className="size-4" />
            </Button>
          }
          title={`Remove ${member.name}`}
          description={`${member.name} leaves ${team.name} but stays in the workspace.`}
          confirmLabel="Remove member"
          onConfirm={() => saveTeam({ id: team.id, removeMembers: [member.id] })}
        />
      }
    >
      <Avatar id={member.id} name={member.name} size="sm" />
      <ListRowText primary={member.name} secondary={member.email} />
      {member.owner && <Badge variant="muted">Owner</Badge>}
    </ListRow>
  );
}
