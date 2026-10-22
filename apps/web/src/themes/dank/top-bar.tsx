import { useQuery } from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";

import { InboxIcon, LogOutIcon, SettingsIcon, UserCircleIcon } from "@/components/icons";
import { useInboxUnreadCount } from "@/components/inbox/inbox-unread-badge";
import { UserAvatar } from "@/components/issues/pickers";
import { GlobalSearch } from "@/components/search/global-search";
import { Button, LinkButton } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { SimpleTooltip } from "@/components/ui/tooltip";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { meQuery, signOut } from "@/lib/queries/me";

export function DankTopBar() {
  return (
    <header className="hidden h-12 shrink-0 items-center gap-2 md:flex">
      <div className="w-24 shrink-0" />
      <div className="flex min-w-0 flex-1 items-center justify-center gap-2">
        <GlobalSearch />
        <AccountMenu />
      </div>
      <div className="flex w-24 shrink-0 items-center justify-end gap-2">
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
    <DropdownMenu>
      <DropdownMenuTrigger
        render={
          <Button
            variant="ghost"
            size="icon"
            aria-label="Account"
            className="dank:size-12 dank:rounded-full shrink-0"
          >
            <UserAvatar user={me} size="lg" />
          </Button>
        }
      />
      <DropdownMenuContent align="end" className="w-64">
        <DropdownMenuGroup>
          <DropdownMenuLabel className="flex flex-col gap-0.5">
            <span className="text-foreground truncate text-sm">{me.name}</span>
            <span className="truncate">{me.email}</span>
          </DropdownMenuLabel>
        </DropdownMenuGroup>
        <DropdownMenuSeparator />
        <DropdownMenuGroup>
          <DropdownMenuItem onClick={() => void signOut()}>
            <LogOutIcon className="size-4" />
            Sign out
          </DropdownMenuItem>
        </DropdownMenuGroup>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

function InboxMenu() {
  const { workspace } = useWorkspace();
  const navigate = useNavigate();
  const unread = useInboxUnreadCount();
  const params = { workspace: workspace.urlKey };
  const unreadLabel = unread > 99 ? "99+" : String(unread);
  return (
    <DropdownMenu>
      <SimpleTooltip content="Inbox and my issues">
        <DropdownMenuTrigger
          render={
            <Button
              variant="ghost"
              size="icon"
              aria-label={unread > 0 ? `Inbox, ${unread} unread` : "Inbox"}
              className="text-muted-foreground dank:size-10"
            >
              <InboxIcon className="size-5" />
              {unread > 0 && (
                <span className="bg-destructive text-destructive-foreground absolute -top-0.5 -right-0.5 flex h-4 min-w-4 items-center justify-center rounded-full px-1 text-xs leading-none tabular-nums">
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
        className="text-muted-foreground dank:size-10"
      >
        <SettingsIcon className="size-5" />
      </LinkButton>
    </SimpleTooltip>
  );
}
