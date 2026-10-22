import { Link, type LinkProps, useNavigate } from "@tanstack/react-router";
import { type ComponentProps, type ReactNode } from "react";

import { useCommandPanelStore } from "@/components/command-panel/command-panel-store";
import {
  BoxIcon,
  BuildingIcon,
  CircleDotIcon,
  EllipsisIcon,
  FileTextIcon,
  InboxIcon,
  KeyboardIcon,
  LayersIcon,
  MenuIcon,
  MessagesSquareIcon,
  RefreshCwIcon,
  RocketIcon,
  SearchIcon,
  SettingsIcon,
  SquarePenIcon,
  TargetIcon,
  type TIcon,
  TriangleIcon,
  UserCircleIcon,
} from "@/components/icons/symbols.gen";
import { useInboxUnreadCount } from "@/components/inbox/inbox-unread-badge";
import { useNewIssueStore } from "@/components/issues/new-issue-store";
import { SidebarToggleTooltip } from "@/components/layout/sidebar-button";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { ScrollArea } from "@/components/ui/scroll-area";
import { SimpleTooltip } from "@/components/ui/tooltip";
import { useWorkspaceTeam } from "@/components/workspace/teams-provider";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { WorkspaceSwitcher } from "@/components/workspace/workspace-switcher";
import { useSidebarStore } from "@/lib/stores/sidebar";

type TDestination = {
  label: string;
  icon: TIcon;
  link: LinkProps;
  badge?: ReactNode;
  isHidden?: boolean;
};

/** The Material 3 navigation rail: the main destinations, with the rest in menus. */
export function NavigationRail() {
  const { workspace, canEdit, isAdmin } = useWorkspace();
  const team = useWorkspaceTeam();
  const openNewIssue = useNewIssueStore((state) => state.open);
  const openCommandPanel = useCommandPanelStore((state) => state.setOpen);
  const expand = useSidebarStore((state) => state.toggle);
  const unread = useInboxUnreadCount();
  const params = { workspace: workspace.urlKey };

  const destinations: TDestination[] = [
    {
      label: "Inbox",
      icon: InboxIcon,
      link: { to: "/$workspace/inbox", params },
      badge: unread > 0 && <RailBadge count={unread} />,
    },
    { label: "My issues", icon: UserCircleIcon, link: { to: "/$workspace/my-issues", params } },
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
    { label: "Docs", icon: FileTextIcon, link: { to: "/$workspace/documents", params } },
    { label: "Forum", icon: MessagesSquareIcon, link: { to: "/$workspace/forum", params } },
  ];

  return (
    <nav aria-label="Workspace" className="bg-sidebar flex h-full w-24 flex-col items-center">
      <div className="flex flex-col items-center gap-3 pt-3 pb-4">
        <SidebarToggleTooltip label="Expand navigation">
          <Button
            variant="ghost"
            size="icon"
            aria-label="Expand navigation"
            className="text-muted-foreground size-10"
            onClick={expand}
          >
            <MenuIcon className="size-5" />
          </Button>
        </SidebarToggleTooltip>
        <WorkspaceSwitcher compact />
        {canEdit && (
          <SimpleTooltip content="New issue" side="right">
            <Button
              variant="tonal"
              size="icon"
              aria-label="New issue"
              className="size-14 rounded-lg"
              onClick={() => openNewIssue()}
            >
              <SquarePenIcon className="size-5" />
            </Button>
          </SimpleTooltip>
        )}
      </div>
      <ScrollArea className="min-h-0 w-full flex-1" classNameViewport="flex flex-col items-center">
        <div className="flex w-full flex-col items-center gap-1">
          {destinations
            .filter((destination) => !destination.isHidden)
            .map((destination) => (
              <RailLink key={destination.label} {...destination} />
            ))}
          <MoreMenu />
        </div>
      </ScrollArea>
      <div className="flex w-full flex-col items-center gap-1 pt-2 pb-3">
        <RailButton label="Search" icon={SearchIcon} onClick={() => openCommandPanel(true)} />
        <RailLink
          label="Settings"
          icon={SettingsIcon}
          link={{
            to: isAdmin ? "/$workspace/settings" : "/$workspace/settings/account",
            params,
          }}
        />
      </div>
    </nav>
  );
}

const railItemClassName =
  "group/rail text-muted-foreground has-hover:hover:text-foreground focus-visible:text-foreground data-[status=active]:text-secondary-foreground data-popup-open:text-foreground flex w-full flex-col items-center gap-1 py-1 text-xs font-medium outline-hidden select-none";

function RailIndicator({ children }: { children: ReactNode }) {
  return (
    <span className="group-has-hover/rail:group-hover/rail:bg-accent group-focus-visible/rail:ring-primary group-data-[status=active]/rail:bg-secondary group-data-popup-open/rail:bg-accent relative flex h-8 w-14 items-center justify-center rounded-full transition-colors group-focus-visible/rail:ring-2">
      {children}
    </span>
  );
}

function RailIcon({ icon: Icon, isActive }: { icon: TIcon; isActive?: boolean }) {
  return <Icon filled={isActive} className="size-5" />;
}

function RailLink({ label, icon, link, badge }: TDestination) {
  return (
    <Link {...link} activeOptions={{ includeSearch: false }} className={railItemClassName}>
      {({ isActive }) => (
        <>
          <RailIndicator>
            <RailIcon icon={icon} isActive={isActive} />
            {badge}
          </RailIndicator>
          <span className="max-w-full truncate px-1">{label}</span>
        </>
      )}
    </Link>
  );
}

function RailButton({
  label,
  icon,
  isActive,
  ...props
}: ComponentProps<typeof Button> & { label: string; icon: TIcon; isActive?: boolean }) {
  return (
    <Button
      variant="ghost"
      forceMinSize={false}
      data-status={isActive ? "active" : undefined}
      {...props}
      className={`${railItemClassName} h-auto rounded-none px-0 active:rounded-none active:bg-transparent has-hover:hover:bg-transparent`}
    >
      <RailIndicator>
        <RailIcon icon={icon} isActive={isActive} />
      </RailIndicator>
      <span className="max-w-full truncate px-1">{label}</span>
    </Button>
  );
}

function RailBadge({ count }: { count: number }) {
  return (
    <span
      aria-label={`${count} unread`}
      className="bg-destructive text-destructive-foreground absolute top-0 right-2 flex h-4 min-w-4 items-center justify-center rounded-full px-1 text-xs leading-none tabular-nums"
    >
      {count > 99 ? "99+" : count}
    </span>
  );
}

function MoreMenu() {
  const { workspace, canEdit } = useWorkspace();
  const openShortcuts = useCommandPanelStore((state) => state.setShortcutsOpen);
  const navigate = useNavigate();
  const params = { workspace: workspace.urlKey };
  const pages: TDestination[] = [
    {
      label: "Initiatives",
      icon: TargetIcon,
      link: { to: "/$workspace/initiatives", params },
      isHidden: !canEdit,
    },
    {
      label: "Customers",
      icon: BuildingIcon,
      link: { to: "/$workspace/customers", params },
      isHidden: !canEdit,
    },
    { label: "Releases", icon: RocketIcon, link: { to: "/$workspace/releases", params } },
  ];
  return (
    <DropdownMenu>
      <DropdownMenuTrigger render={<RailButton label="More" icon={EllipsisIcon} />} />
      <DropdownMenuContent side="right" className="w-56">
        <DropdownMenuGroup>
          {pages
            .filter((page) => !page.isHidden)
            .map((page) => (
              <DropdownMenuItem key={page.label} onClick={() => void navigate(page.link)}>
                <page.icon className="size-4" />
                {page.label}
              </DropdownMenuItem>
            ))}
        </DropdownMenuGroup>
        <DropdownMenuSeparator />
        <DropdownMenuGroup>
          <DropdownMenuItem onClick={() => openShortcuts(true)}>
            <KeyboardIcon className="size-4" />
            Keyboard shortcuts
          </DropdownMenuItem>
        </DropdownMenuGroup>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
