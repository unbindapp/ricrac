import { useQuery } from "@tanstack/react-query";
import { CircleUserIcon, UsersIcon } from "lucide-react";

import type { UserRef, WorkspaceUser } from "@/api/ops.gen";
import { PickerTrigger, type TPickerAppearance } from "@/components/issues/pickers";
import { PropertyPicker, type TPickerOption } from "@/components/issues/property-picker";
import { Avatar } from "@/components/shared/avatar";
import type { TUserSummary } from "@/components/shared/user-chip";
import type { TAvatarSize } from "@/components/ui/avatar";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { usersQuery } from "@/lib/queries/users";

const NONE = "__none__";

/** Active members of the workspace, for pickers and @mentions. */
export function useWorkspaceUserList() {
  const { workspace } = useWorkspace();
  const { data, isPending } = useQuery(usersQuery(workspace.id));
  return { users: (data ?? []).filter((user) => user.active), isPending };
}

export function toUserRef(user: WorkspaceUser): UserRef {
  return { id: user.id, name: user.name, displayName: user.displayName, email: user.email };
}

/** The people the Markdown editor offers after @. */
export function useMentionUsers(): TUserSummary[] {
  const { users } = useWorkspaceUserList();
  return users.map((user) => ({
    id: user.id,
    name: user.name,
    display_name: user.displayName,
    avatar_url: user.avatarUrl,
  }));
}

/** A person's avatar; references carry no picture, so it comes from the member list. */
export function PersonAvatar({ user, size = "sm" }: { user: UserRef; size?: TAvatarSize }) {
  const { users } = useWorkspaceUserList();
  const avatarUrl = users.find((member) => member.id === user.id)?.avatarUrl;
  return <Avatar id={user.id} name={user.name} src={avatarUrl} size={size} />;
}

export function AvatarStack({ users, max = 3 }: { users: UserRef[]; max?: number }) {
  const extra = users.length - max;
  return (
    <span className="flex shrink-0 items-center -space-x-1">
      {users.slice(0, max).map((user) => (
        <PersonAvatar key={user.id} user={user} size="xs" />
      ))}
      {extra > 0 && <span className="text-muted-foreground pl-2 text-xs">+{extra}</span>}
    </span>
  );
}

function userOption(user: WorkspaceUser): TPickerOption {
  return {
    value: user.id,
    label: user.name,
    description: user.isMe ? "You" : undefined,
    keywords: [user.displayName, user.email],
    icon: <Avatar id={user.id} name={user.name} src={user.avatarUrl} size="xs" />,
  };
}

type TPersonPickerProps = {
  /** Lead, Owner: what the person is to the thing. */
  role: string;
  user: UserRef | null | undefined;
  onChange: (userId: string | null) => void;
  appearance?: TPickerAppearance;
  disabled?: boolean;
};

export function PersonPicker({
  role,
  user,
  onChange,
  appearance = "row",
  disabled,
}: TPersonPickerProps) {
  const { users, isPending } = useWorkspaceUserList();
  const options: TPickerOption[] = [
    {
      value: NONE,
      label: `No ${role.toLowerCase()}`,
      icon: <CircleUserIcon className="text-muted-foreground size-4" />,
    },
    ...users.map(userOption),
  ];
  return (
    <PropertyPicker
      label={`Set ${role.toLowerCase()}`}
      options={options}
      value={[user?.id ?? NONE]}
      isPending={isPending}
      disabled={disabled}
      onChange={([id]) => {
        if (id === (user?.id ?? NONE)) return;
        onChange(id === NONE ? null : id);
      }}
      trigger={
        <PickerTrigger
          appearance={appearance}
          label={user ? `${role}: ${user.name}` : `Set ${role.toLowerCase()}`}
          icon={
            user ? (
              <PersonAvatar user={user} size="xs" />
            ) : (
              <CircleUserIcon className="text-muted-foreground size-4" />
            )
          }
          text={user?.name ?? (appearance === "row" ? `No ${role.toLowerCase()}` : role)}
          muted={!user}
        />
      }
    />
  );
}

export function PeoplePicker({
  people,
  onChange,
  appearance = "row",
  disabled,
}: {
  people: UserRef[];
  onChange: (ids: string[]) => void;
  appearance?: TPickerAppearance;
  disabled?: boolean;
}) {
  const { users, isPending } = useWorkspaceUserList();
  const count = people.length;
  return (
    <PropertyPicker
      label="Change members"
      multiple
      options={users.map(userOption)}
      value={people.map((person) => person.id)}
      isPending={isPending}
      disabled={disabled}
      onChange={onChange}
      trigger={
        <PickerTrigger
          appearance={appearance}
          label="Change members"
          icon={
            count > 0 ? (
              <AvatarStack users={people} />
            ) : (
              <UsersIcon className="text-muted-foreground size-4" />
            )
          }
          text={count === 0 ? (appearance === "row" ? "No members" : "Members") : memberText(count)}
          muted={count === 0}
        />
      }
    />
  );
}

function memberText(count: number) {
  return `${count} member${count === 1 ? "" : "s"}`;
}
