import {
  BoxIcon,
  BuildingIcon,
  CircleDotIcon,
  FileTextIcon,
  InboxIcon,
  KeyboardIcon,
  LayersIcon,
  ListTodoIcon,
  MessagesSquareIcon,
  PlusIcon,
  RefreshCwIcon,
  RocketIcon,
  SearchIcon,
  SettingsIcon,
  SquarePenIcon,
  TargetIcon,
  TriangleIcon,
  UserCircleIcon,
} from "lucide-react";

import type { Team } from "@/api/ops.gen";
import { useCommandPanelStore } from "@/components/command-panel/command-panel-store";
import { SidebarFavorites } from "@/components/favorites/sidebar-favorites";
import { InboxUnreadBadge } from "@/components/inbox/inbox-unread-badge";
import { useNewIssueStore } from "@/components/issues/new-issue-store";
import { useCreateTeamStore } from "@/components/settings/create-team-store";
import { TeamIcon } from "@/components/shared/team-icon";
import { Button, LinkButton } from "@/components/ui/button";
import { ScrollArea } from "@/components/ui/scroll-area";
import { Skeleton } from "@/components/ui/skeleton";
import {
  SidebarCollapsible,
  SidebarNavItem,
  SidebarSection,
} from "@/components/workspace/sidebar-nav";
import { useTeams } from "@/components/workspace/teams-provider";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { WorkspaceSwitcher } from "@/components/workspace/workspace-switcher";

type TProps = {
  onNavigate?: () => void;
};

export function Sidebar({ onNavigate }: TProps) {
  const { workspace, canEdit } = useWorkspace();
  const openCommandPanel = useCommandPanelStore((state) => state.setOpen);
  const openNewIssue = useNewIssueStore((state) => state.open);
  const openCreateTeam = useOpenCreateTeam(onNavigate);
  const params = { workspace: workspace.urlKey };

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
            onClick={() => {
              onNavigate?.();
              openNewIssue();
            }}
          >
            <SquarePenIcon className="size-4" />
          </Button>
        )}
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
            <SidebarNavItem
              label="Projects"
              icon={BoxIcon}
              link={{ to: "/$workspace/projects", params }}
              onNavigate={onNavigate}
            />
            <SidebarNavItem
              label="Views"
              icon={LayersIcon}
              link={{ to: "/$workspace/views", params }}
              onNavigate={onNavigate}
            />
            {canEdit && (
              <SidebarNavItem
                label="Initiatives"
                icon={TargetIcon}
                link={{ to: "/$workspace/initiatives", params }}
                onNavigate={onNavigate}
              />
            )}
            <SidebarNavItem
              label="Documents"
              icon={FileTextIcon}
              link={{ to: "/$workspace/documents", params }}
              onNavigate={onNavigate}
            />
            <SidebarNavItem
              label="Forum"
              icon={MessagesSquareIcon}
              link={{ to: "/$workspace/forum", params }}
              onNavigate={onNavigate}
            />
            {canEdit && (
              <SidebarNavItem
                label="Customers"
                icon={BuildingIcon}
                link={{ to: "/$workspace/customers", params }}
                onNavigate={onNavigate}
              />
            )}
            <SidebarNavItem
              label="Releases"
              icon={RocketIcon}
              link={{ to: "/$workspace/releases", params }}
              onNavigate={onNavigate}
            />
            <SidebarNavItem
              label="All issues"
              icon={ListTodoIcon}
              link={{ to: "/$workspace/issues", params }}
              onNavigate={onNavigate}
            />
          </SidebarSection>
          <SidebarFavorites onNavigate={onNavigate} />
          <SidebarSection
            title="Your teams"
            action={
              canEdit && (
                <Button
                  variant="ghost"
                  size="icon"
                  forceMinSize={false}
                  aria-label="New team"
                  onClick={openCreateTeam}
                  className="text-muted-foreground size-6"
                >
                  <PlusIcon className="size-3.5" />
                </Button>
              )
            }
          >
            <TeamsNav onNavigate={onNavigate} />
          </SidebarSection>
        </nav>
      </ScrollArea>
      <SidebarFooter onNavigate={onNavigate} />
    </div>
  );
}

function useOpenCreateTeam(onNavigate?: () => void) {
  const setOpen = useCreateTeamStore((state) => state.setOpen);
  return () => {
    onNavigate?.();
    setOpen(true);
  };
}

function TeamsNav({ onNavigate }: TProps) {
  const { canEdit } = useWorkspace();
  const { myTeams, isPending, error, refetch } = useTeams();
  const openCreateTeam = useOpenCreateTeam(onNavigate);
  if (isPending) {
    return (
      <div className="flex flex-col gap-2 px-2.5 py-1">
        <Skeleton className="h-3 w-2/3" />
        <Skeleton className="h-3 w-1/2" />
      </div>
    );
  }
  if (error) {
    return (
      <Button
        variant="ghost"
        size="sm"
        onClick={refetch}
        className="text-destructive justify-start"
      >
        Could not load teams. Retry
      </Button>
    );
  }
  if (myTeams.length === 0 && canEdit) {
    return (
      <Button
        variant="ghost"
        size="sm"
        onClick={openCreateTeam}
        className="text-muted-foreground justify-start font-normal"
      >
        <PlusIcon className="size-4" />
        Create a team
      </Button>
    );
  }
  if (myTeams.length === 0) {
    return <p className="text-muted-foreground px-2.5 text-xs">You are not in any team yet.</p>;
  }
  return myTeams.map((team) => <TeamNav key={team.id} team={team} onNavigate={onNavigate} />);
}

function TeamNav({ team, onNavigate }: { team: Team } & TProps) {
  const { workspace } = useWorkspace();
  const params = { workspace: workspace.urlKey, key: team.key };
  return (
    <SidebarCollapsible
      label={team.name}
      icon={<TeamIcon teamKey={team.key} icon={team.icon} color={team.color} size="sm" />}
    >
      <SidebarNavItem
        label="Issues"
        icon={CircleDotIcon}
        link={{ to: "/$workspace/team/$key/issues", params }}
        onNavigate={onNavigate}
      />
      <SidebarNavItem
        label="Backlog"
        icon={ListTodoIcon}
        link={{ to: "/$workspace/team/$key/backlog", params }}
        onNavigate={onNavigate}
      />
      {team.triageEnabled && (
        <SidebarNavItem
          label="Triage"
          icon={TriangleIcon}
          link={{ to: "/$workspace/team/$key/triage", params }}
          onNavigate={onNavigate}
        />
      )}
      {team.cyclesEnabled && (
        <SidebarNavItem
          label="Cycles"
          icon={RefreshCwIcon}
          link={{ to: "/$workspace/team/$key/cycles", params }}
          onNavigate={onNavigate}
        />
      )}
      <SidebarNavItem
        label="Projects"
        icon={BoxIcon}
        link={{ to: "/$workspace/team/$key/projects", params }}
        onNavigate={onNavigate}
      />
    </SidebarCollapsible>
  );
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
