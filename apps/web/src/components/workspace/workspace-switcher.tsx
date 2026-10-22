import { useQuery } from "@tanstack/react-query";
import { type LinkProps, useNavigate } from "@tanstack/react-router";
import { useState } from "react";
import {
  CheckIcon,
  ChevronDownIcon,
  DoorOpenIcon,
  LogOutIcon,
  MailIcon,
  PlusIcon,
  SettingsIcon,
  UserIcon,
  UserPlusIcon,
} from "lucide-react";

import { useInviteStore } from "@/components/settings/invite-store";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { LeaveWorkspaceDialog } from "@/components/workspace/leave-workspace-dialog";
import { WorkspaceIcon } from "@/components/workspace/workspace-icon";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { meQuery, signOut } from "@/lib/queries/me";

export function WorkspaceSwitcher({ onNavigate }: { onNavigate?: () => void }) {
  const { workspace, isAdmin, canEdit } = useWorkspace();
  const openInvite = useInviteStore((state) => state.setOpen);
  const { data: me } = useQuery(meQuery);
  const navigate = useNavigate();
  const [isLeaving, setIsLeaving] = useState(false);
  const params = { workspace: workspace.urlKey };
  const invitationCount = me?.invitations.length ?? 0;

  const go = (link: LinkProps) => {
    onNavigate?.();
    void navigate(link);
  };

  return (
    <>
      <DropdownMenu>
        <DropdownMenuTrigger
          render={
            <Button
              variant="ghost"
              size="sm"
              forceMinSize={false}
              aria-label="Switch workspace"
              className="h-8 min-w-0 flex-1 justify-start px-1.5"
            >
              <WorkspaceIcon name={workspace.name} logoUrl={workspace.logoUrl} />
              <span className="min-w-0 truncate">{workspace.name}</span>
              <ChevronDownIcon className="text-muted-foreground size-3.5" />
              <span className="flex-1" />
            </Button>
          }
        />
        <DropdownMenuContent className="w-64">
          <DropdownMenuGroup>
            <DropdownMenuLabel>{me?.email}</DropdownMenuLabel>
            {me?.workspaces.map((item) => (
              <DropdownMenuItem
                key={item.id}
                onClick={() => go({ to: "/$workspace", params: { workspace: item.urlKey } })}
              >
                <WorkspaceIcon
                  name={item.name}
                  logoUrl={item.id === workspace.id ? workspace.logoUrl : item.logoUrl}
                />
                <span className="min-w-0 flex-1 truncate">{item.name}</span>
                {item.id === workspace.id && <CheckIcon className="size-4" />}
              </DropdownMenuItem>
            ))}
          </DropdownMenuGroup>
          <DropdownMenuSeparator />
          <DropdownMenuGroup>
            {canEdit && (
              <DropdownMenuItem
                onClick={() => {
                  onNavigate?.();
                  openInvite(true);
                }}
              >
                <UserPlusIcon className="size-4" />
                Invite people
              </DropdownMenuItem>
            )}
            {isAdmin && (
              <DropdownMenuItem onClick={() => go({ to: "/$workspace/settings", params })}>
                <SettingsIcon className="size-4" />
                Workspace settings
              </DropdownMenuItem>
            )}
            <DropdownMenuItem onClick={() => go({ to: "/$workspace/settings/account", params })}>
              <UserIcon className="size-4" />
              Account settings
            </DropdownMenuItem>
            <DropdownMenuItem onClick={() => go({ to: "/create-workspace" })}>
              {invitationCount > 0 ? (
                <MailIcon className="size-4" />
              ) : (
                <PlusIcon className="size-4" />
              )}
              <span className="flex-1">
                {invitationCount > 0 ? "Join or create workspace" : "Create workspace"}
              </span>
              {invitationCount > 0 && <Badge variant="muted">{invitationCount}</Badge>}
            </DropdownMenuItem>
            <DropdownMenuItem onClick={() => setIsLeaving(true)}>
              <DoorOpenIcon className="size-4" />
              Leave workspace
            </DropdownMenuItem>
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
      <LeaveWorkspaceDialog isOpen={isLeaving} onOpenChange={setIsLeaving} />
    </>
  );
}
