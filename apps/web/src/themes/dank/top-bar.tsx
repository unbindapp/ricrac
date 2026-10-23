import { useQuery } from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";

import { InboxIcon, SettingsIcon, UserCircleIcon } from "@/components/icons";
import { useInboxUnreadCount } from "@/components/inbox/inbox-unread-badge";
import { UserAvatar } from "@/components/issues/pickers";
import { GlobalSearch } from "@/themes/dank/global-search";
import { Button, LinkButton } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { SimpleTooltip } from "@/components/ui/tooltip";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { WorkspaceMenu } from "@/components/workspace/workspace-switcher";
import { meQuery } from "@/lib/queries/me";

/** Material 3's app bar with search: a 56dp search bar in a 64dp bar, actions at the end. */
export function DankTopBar() {
  return (
    <header className="-mr-5 hidden h-16 shrink-0 items-center gap-1 md:flex">
      <div className="w-24 shrink-0" />
      <div className="flex min-w-0 flex-1 items-center justify-center py-1">
        <GlobalSearch trailing={<AccountMenu />} />
      </div>
      <div className="flex w-24 shrink-0 items-center justify-end gap-2 pr-1">
        <InboxMenu />
        <SettingsButton />
      </div>
    </header>
  );
}

function AccountMenu() {
  const { data: me } = useQuery(meQuery);
  if (!me) return null;
  return (
    <WorkspaceMenu
      align="end"
      header={
        <DropdownMenuLabel className="flex flex-col gap-0.5">
          <span className="text-foreground truncate text-sm">{me.name}</span>
          <span className="truncate">{me.email}</span>
        </DropdownMenuLabel>
      }
      trigger={
        <Button
          variant="ghost"
          size="icon"
          aria-label="Account and workspaces"
          className="shrink-0 rounded-full active:rounded-full data-popup-open:rounded-full [&>*]:size-7.5"
        >
          <UserAvatar user={me} size="lg" />
        </Button>
      }
    />
  );
}

function InboxMenu() {
  const { workspace } = useWorkspace();
  const navigate = useNavigate();
  const unread = useInboxUnreadCount();
  const params = { workspace: workspace.urlKey };
  const unreadLabel = unread > 999 ? "999+" : String(unread);
  return (
    <DropdownMenu>
      <SimpleTooltip content="Inbox and my issues">
        <DropdownMenuTrigger
          render={
            <Button
              variant="ghost"
              size="icon"
              aria-label={unread > 0 ? `Inbox, ${unread} unread` : "Inbox"}
            >
              <InboxIcon />
              {unread > 0 && (
                <span className="bg-m3-error text-m3-on-error type-label-small absolute top-1.5 left-5 flex h-4 min-w-4 items-center justify-center rounded-full px-1 tabular-nums">
                  {unreadLabel}
                </span>
              )}
            </Button>
          }
        />
      </SimpleTooltip>
      <DropdownMenuContent align="end" className="w-56">
        <DropdownMenuGroup>
          <DropdownMenuItem onClick={() => void navigate({ to: "/$workspace/inbox", params })}>
            <InboxIcon className="size-4" />
            <span className="flex-1">Inbox</span>
            {unread > 0 && (
              <span className="text-muted-foreground text-xs tabular-nums">{unreadLabel}</span>
            )}
          </DropdownMenuItem>
          <DropdownMenuItem onClick={() => void navigate({ to: "/$workspace/my-issues", params })}>
            <UserCircleIcon className="size-4" />
            My issues
          </DropdownMenuItem>
        </DropdownMenuGroup>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

function SettingsButton() {
  const { workspace, isAdmin } = useWorkspace();
  return (
    <SimpleTooltip content="Settings">
      <LinkButton
        to={isAdmin ? "/$workspace/settings" : "/$workspace/settings/account"}
        params={{ workspace: workspace.urlKey }}
        variant="ghost"
        size="icon"
        aria-label="Settings"
      >
        <SettingsIcon />
      </LinkButton>
    </SimpleTooltip>
  );
}
