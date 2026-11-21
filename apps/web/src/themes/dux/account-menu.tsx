import { Menu } from "@base-ui/react/menu";
import { useQuery } from "@tanstack/react-query";
import { type LinkProps, useNavigate } from "@tanstack/react-router";
import { type ComponentType, useState } from "react";

import type { Me } from "@/api/account.gen";
import {
  CheckIcon,
  DoorClosedIcon,
  LogOutIcon,
  MailIcon,
  PlusIcon,
  SettingsIcon,
  SquareUserIcon,
  UserIcon,
  UserPlusIcon,
  UsersRoundIcon,
  XIcon,
} from "@/components/icons";
import { useInviteStore } from "@/components/settings/invite-store";
import { Button } from "@/components/ui/button";
import { SimpleTooltip } from "@/components/ui/tooltip";
import { LeaveWorkspaceDialog } from "@/components/workspace/leave-workspace-dialog";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { initials } from "@/lib/helpers/colors";
import { meQuery, signOut } from "@/lib/queries/me";

type TAction = {
  label: string;
  icon: ComponentType;
  onClick: () => void;
  isHidden?: boolean;
  badge?: number;
};

/** The avatar top right, opening the account, its workspaces and what to do with them. */
export function AccountMenu() {
  const { data: me } = useQuery(meQuery);
  const [isOpen, setIsOpen] = useState(false);
  const [isLeaving, setIsLeaving] = useState(false);
  if (!me) return null;
  return (
    <>
      <Menu.Root open={isOpen} onOpenChange={setIsOpen}>
        <SimpleTooltip content="Account" side="bottom">
          <Menu.Trigger
            render={
              <Button
                variant="ghost"
                size="icon"
                forceMinSize={false}
                aria-label="Account"
                data-slot="dux-account-button"
              />
            }
          >
            <Pfp me={me} />
          </Menu.Trigger>
        </SimpleTooltip>
        <Menu.Portal>
          <Menu.Positioner
            side="bottom"
            align="end"
            sideOffset={8}
            collisionPadding={16}
            className="isolate z-1000 outline-none"
          >
            <Menu.Popup data-slot="dux-account-menu" aria-label="Account">
              <div data-slot="dux-account-top">
                <Button
                  variant="ghost"
                  size="icon"
                  forceMinSize={false}
                  aria-label="Close"
                  onClick={() => setIsOpen(false)}
                >
                  <XIcon />
                </Button>
              </div>
              <AccountMenuBody
                me={me}
                onClose={() => setIsOpen(false)}
                onLeave={() => setIsLeaving(true)}
              />
            </Menu.Popup>
          </Menu.Positioner>
        </Menu.Portal>
      </Menu.Root>
      <LeaveWorkspaceDialog isOpen={isLeaving} onOpenChange={setIsLeaving} />
    </>
  );
}

function AccountMenuBody({
  me,
  onClose,
  onLeave,
}: {
  me: Me;
  onClose: () => void;
  onLeave: () => void;
}) {
  const { workspace, isAdmin, canEdit } = useWorkspace();
  const openInvite = useInviteStore((state) => state.setOpen);
  const navigate = useNavigate();
  const params = { workspace: workspace.urlKey };
  const invitationCount = me.invitations.length;

  const go = (link: LinkProps) => {
    onClose();
    void navigate(link);
  };

  const actions: TAction[] = [
    {
      label: "My issues",
      icon: SquareUserIcon,
      onClick: () => go({ to: "/$workspace/my-issues", params }),
    },
    {
      label: "Settings",
      icon: SettingsIcon,
      onClick: () => go({ to: "/$workspace/settings", params }),
      isHidden: !isAdmin,
    },
    {
      label: "Invite people",
      icon: UserPlusIcon,
      onClick: () => {
        onClose();
        openInvite(true);
      },
      isHidden: !canEdit,
    },
    {
      label: "Account settings",
      icon: UserIcon,
      onClick: () => go({ to: "/$workspace/settings/account", params }),
    },
    {
      label: invitationCount > 0 ? "Join or create workspace" : "Create workspace",
      icon: invitationCount > 0 ? MailIcon : PlusIcon,
      onClick: () => go({ to: "/create-workspace" }),
      badge: invitationCount,
    },
    { label: "Leave workspace", icon: DoorClosedIcon, onClick: onLeave },
  ];

  return (
    <>
      <div data-slot="dux-account-group" role="none">
        <div data-slot="dux-account-row" data-head="" role="none">
          <Pfp me={me} />
          <span data-slot="dux-account-who">
            <b>{me.name}</b>
            <span>{me.email}</span>
          </span>
        </div>
      </div>
      <Menu.RadioGroup
        value={workspace.id}
        data-slot="dux-account-group"
        aria-label="Workspaces"
        onValueChange={(id) => {
          const picked = me.workspaces.find((item) => item.id === id);
          if (picked) go({ to: "/$workspace", params: { workspace: picked.urlKey } });
        }}
      >
        {me.workspaces.map((item) => {
          const logoUrl = item.id === workspace.id ? workspace.logoUrl : item.logoUrl;
          return (
            <Menu.RadioItem key={item.id} value={item.id} closeOnClick data-slot="dux-account-row">
              <span data-slot="dux-workspace-icon">
                {logoUrl ? <img src={logoUrl} alt="" /> : <UsersRoundIcon />}
              </span>
              <span data-slot="dux-account-label">{item.name}</span>
              <Menu.RadioItemIndicator data-slot="dux-account-check">
                <CheckIcon />
              </Menu.RadioItemIndicator>
            </Menu.RadioItem>
          );
        })}
      </Menu.RadioGroup>
      <Menu.Group data-slot="dux-account-group">
        {actions
          .filter((action) => !action.isHidden)
          .map((action) => (
            <Menu.Item key={action.label} data-slot="dux-account-row" onClick={action.onClick}>
              <action.icon />
              <span data-slot="dux-account-label">{action.label}</span>
              {Boolean(action.badge) && <span data-slot="dux-badge">{action.badge}</span>}
            </Menu.Item>
          ))}
      </Menu.Group>
      <Menu.Group data-slot="dux-account-group">
        <Menu.Item data-slot="dux-account-row" onClick={() => void signOut()}>
          <LogOutIcon />
          <span data-slot="dux-account-label">Sign out</span>
        </Menu.Item>
      </Menu.Group>
    </>
  );
}

function Pfp({ me }: { me: Me }) {
  return (
    <span data-slot="dux-pfp">
      {me.avatarUrl ? <img src={me.avatarUrl} alt="" /> : initials(me.name).slice(0, 1)}
    </span>
  );
}
