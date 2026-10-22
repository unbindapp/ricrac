import type { LinkProps } from "@tanstack/react-router";
import type { ReactNode } from "react";

import { useCommandPanelStore } from "@/components/command-panel/command-panel-store";
import { SidebarFavorites } from "@/components/favorites/sidebar-favorites";
import {
  BoxIcon,
  BuildingIcon,
  CircleDotIcon,
  FileTextIcon,
  InboxIcon,
  KeyboardIcon,
  LayersIcon,
  type LucideIcon,
  MessagesSquareIcon,
  RefreshCwIcon,
  RocketIcon,
  SearchIcon,
  SettingsIcon,
  SquarePenIcon,
  TargetIcon,
  TriangleIcon,
  UserCircleIcon,
} from "@/components/icons";
import { InboxUnreadBadge } from "@/components/inbox/inbox-unread-badge";
import { useNewIssueStore } from "@/components/issues/new-issue-store";
import { Button, LinkButton } from "@/components/ui/button";
import { ScrollArea } from "@/components/ui/scroll-area";
import { SidebarNavItem, SidebarSection } from "@/components/workspace/sidebar-nav";
import { useWorkspaceTeam } from "@/components/workspace/teams-provider";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { WorkspaceSwitcher } from "@/components/workspace/workspace-switcher";

type TProps = {
  onNavigate?: () => void;
};

type TSidebarProps = TProps & {
  hideButton?: ReactNode;
};

type TNavDestination = { label: string; icon: LucideIcon; link: LinkProps; isHidden?: boolean };

export function Sidebar({ onNavigate, hideButton }: TSidebarProps) {
  const { workspace, canEdit } = useWorkspace();
  const openCommandPanel = useCommandPanelStore((state) => state.setOpen);
  const openNewIssue = useNewIssueStore((state) => state.open);
  const team = useWorkspaceTeam();
  const params = { workspace: workspace.urlKey };
  const destinations: TNavDestination[] = [
    { label: "Issues", icon: CircleDotIcon, link: { to: "/$workspace/issues", params } },
    {
      label: "Triage",
      icon: TriangleIcon,
      link: { to: "/$workspace/triage", params },
      isHidden: !team?.triageEnabled,
    },
    {
      label: "Cycles",
      icon: RefreshCwIcon,
      link: { to: "/$workspace/cycles", params },
      isHidden: !team?.cyclesEnabled,
    },
    { label: "Projects", icon: BoxIcon, link: { to: "/$workspace/projects", params } },
    { label: "Views", icon: LayersIcon, link: { to: "/$workspace/views", params } },
    {
      label: "Initiatives",
      icon: TargetIcon,
      link: { to: "/$workspace/initiatives", params },
      isHidden: !canEdit,
    },
    { label: "Documents", icon: FileTextIcon, link: { to: "/$workspace/documents", params } },
    { label: "Forum", icon: MessagesSquareIcon, link: { to: "/$workspace/forum", params } },
    {
      label: "Customers",
      icon: BuildingIcon,
      link: { to: "/$workspace/customers", params },
      isHidden: !canEdit,
    },
    { label: "Releases", icon: RocketIcon, link: { to: "/$workspace/releases", params } },
  ];

  return (
    <div className="bg-sidebar flex h-full w-full flex-col">
      <div className="flex h-12 items-center gap-1 px-2">
        <WorkspaceSwitcher onNavigate={onNavigate} />
        <Button
          variant="ghost"
          size="icon"
          aria-label="Search"
          onClick={() => {
            onNavigate?.();
            openCommandPanel(true);
          }}
          className="text-muted-foreground"
        >
          <SearchIcon className="size-4" />
        </Button>
        {canEdit && (
          <Button
            variant="outline"
            size="icon"
            aria-label="New issue"
            className="dank:bg-primary-container dank:text-primary-container-foreground dank:has-hover:hover:bg-primary-container/85 dank:active:bg-primary-container/85 dank:rounded-md dank:border-0"
            onClick={() => {
              onNavigate?.();
              openNewIssue();
            }}
          >
            <SquarePenIcon className="size-4" />
          </Button>
        )}
        {hideButton}
      </div>
      <ScrollArea className="min-h-0" classNameViewport="px-2 pb-2">
        <nav aria-label="Workspace" className="flex flex-col gap-4">
          <div className="flex flex-col">
            <SidebarNavItem
              label="Inbox"
              icon={InboxIcon}
              link={{ to: "/$workspace/inbox", params }}
              badge={<InboxUnreadBadge />}
              onNavigate={onNavigate}
            />
            <SidebarNavItem
              label="My issues"
              icon={UserCircleIcon}
              link={{ to: "/$workspace/my-issues", params }}
              onNavigate={onNavigate}
            />
          </div>
          <SidebarSection title="Workspace">
            <NavItems items={destinations} onNavigate={onNavigate} />
          </SidebarSection>
          <SidebarFavorites onNavigate={onNavigate} />
        </nav>
      </ScrollArea>
      <SidebarFooter onNavigate={onNavigate} />
    </div>
  );
}

function NavItems({ items, onNavigate }: { items: TNavDestination[] } & TProps) {
  return items
    .filter((item) => !item.isHidden)
    .map((item) => (
      <SidebarNavItem
        key={item.label}
        label={item.label}
        icon={item.icon}
        link={item.link}
        onNavigate={onNavigate}
      />
    ));
}

function SidebarFooter({ onNavigate }: TProps) {
  const { workspace, isAdmin } = useWorkspace();
  const openShortcuts = useCommandPanelStore((state) => state.setShortcutsOpen);
  const params = { workspace: workspace.urlKey };
  return (
    <div className="flex items-center gap-1 p-2">
      <LinkButton
        to={isAdmin ? "/$workspace/settings" : "/$workspace/settings/account"}
        params={params}
        variant="ghost"
        size="icon"
        aria-label="Settings"
        className="text-muted-foreground"
        onClick={onNavigate}
      >
        <SettingsIcon className="size-4" />
      </LinkButton>
      <Button
        variant="ghost"
        size="icon"
        aria-label="Keyboard shortcuts"
        className="text-muted-foreground"
        onClick={() => {
          onNavigate?.();
          openShortcuts(true);
        }}
      >
        <KeyboardIcon className="size-4" />
      </Button>
    </div>
  );
}
