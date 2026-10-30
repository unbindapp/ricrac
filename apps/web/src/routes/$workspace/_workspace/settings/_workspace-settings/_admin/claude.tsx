import { useQuery } from "@tanstack/react-query";
import { createFileRoute } from "@tanstack/react-router";

import type { AssistantSettings } from "@/api/account.gen";
import type { WorkspaceUser } from "@/api/ops.gen";
import { UsersIcon } from "@/components/icons";
import { List, ListRow, ListRowText } from "@/components/layout/list";
import { ListStates } from "@/components/layout/list-states";
import { SettingsPage } from "@/components/layout/settings-page";
import { SettingsSection } from "@/components/layout/settings-section";
import { Avatar } from "@/components/shared/avatar";
import { Badge } from "@/components/ui/badge";
import { Switch } from "@/components/ui/switch";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import {
  assistantSettingsQuery,
  useSaveAssistantSettingsMutation,
} from "@/lib/queries/assistant-settings";
import { usersQuery } from "@/lib/queries/users";

export const Route = createFileRoute(
  "/$workspace/_workspace/settings/_workspace-settings/_admin/claude",
)({
  component: ClaudeSettingsPage,
});

function ClaudeSettingsPage() {
  const { workspace } = useWorkspace();
  const {
    data: settings,
    isPending,
    error,
    refetch,
  } = useQuery(assistantSettingsQuery(workspace.id));
  return (
    <SettingsPage
      title="Claude"
      description="Ask Claude to find, sum up and change work in this workspace."
    >
      <ListStates
        isPending={isPending}
        error={error}
        onRetry={() => void refetch()}
        isEmpty={false}
        empty={null}
        skeletonRows={3}
      >
        {settings && (
          <>
            <EnabledSection settings={settings} />
            <PeopleSection settings={settings} />
          </>
        )}
      </ListStates>
    </SettingsPage>
  );
}

function EnabledSection({ settings }: { settings: AssistantSettings }) {
  const { workspace } = useWorkspace();
  const { mutate: save } = useSaveAssistantSettingsMutation(workspace.id);
  return (
    <SettingsSection
      title="Turn on Claude"
      description="People picked below can ask Claude from the panel beside every page."
      actions={
        <Switch
          aria-label="Turn on Claude"
          checked={settings.enabled}
          onCheckedChange={(enabled) => save({ enabled })}
        />
      }
    />
  );
}

function PeopleSection({ settings }: { settings: AssistantSettings }) {
  const { workspace } = useWorkspace();
  const { data: users = [], isPending, error, refetch } = useQuery(usersQuery(workspace.id));
  const { mutate: save } = useSaveAssistantSettingsMutation(workspace.id);
  const members = users.filter((user) => user.role !== "guest");

  const allow = (user: WorkspaceUser, isAllowed: boolean) => {
    const others = settings.userIds.filter((id) => id !== user.id);
    save({ userIds: isAllowed ? [...others, user.id] : others });
  };

  return (
    <SettingsSection
      title="Who can ask Claude"
      description="The panel only shows for the people picked here. Before every question Claude reads the workspace document titled Assistant, so keep notes for it there."
    >
      <ListStates
        isPending={isPending}
        error={error}
        onRetry={() => void refetch()}
        isEmpty={members.length === 0}
        empty={{ icon: UsersIcon, title: "No members yet" }}
      >
        <List>
          {members.map((user) => (
            <ListRow
              key={user.id}
              muted={!settings.enabled}
              actions={
                <Switch
                  aria-label={`Let ${user.name} ask Claude`}
                  checked={settings.userIds.includes(user.id)}
                  disabled={!settings.enabled}
                  onCheckedChange={(isAllowed) => allow(user, isAllowed)}
                />
              }
            >
              <Avatar id={user.id} name={user.name} src={user.avatarUrl} size="sm" />
              <ListRowText primary={user.name} secondary={user.email} />
              {user.isMe && <Badge variant="muted">You</Badge>}
            </ListRow>
          ))}
        </List>
      </ListStates>
    </SettingsSection>
  );
}
