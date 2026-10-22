import { Link, type LinkProps, useNavigate } from "@tanstack/react-router";
import { type ComponentProps, type ReactNode } from "react";

import { useCommandPanelStore } from "@/components/command-panel/command-panel-store";
import { SidebarFavorites } from "@/components/favorites/sidebar-favorites";
import {
  BoxIcon,
  BuildingIcon,
  CircleDotIcon,
  EllipsisIcon,
  FileTextIcon,
  InboxIcon,
  KeyboardIcon,
  LayersIcon,
  ListIndentDecreaseIcon,
  MenuIcon,
  MessagesSquareIcon,
  RefreshCwIcon,
  RocketIcon,
  SettingsIcon,
  SquarePenIcon,
  TargetIcon,
  type LucideIcon,
  TriangleIcon,
  UserCircleIcon,
} from "@/components/icons";
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
import { cn } from "@/components/ui/utils";
import { useWorkspaceTeam } from "@/components/workspace/teams-provider";
import { useWorkspace } from "@/components/workspace/workspace-provider";

type TDestination = {
  label: string;
  icon: LucideIcon;
  link: LinkProps;
  unread?: number;
  isHidden?: boolean;
};

type TRailProps = {
  isExpanded: boolean;
  menuLabel: string;
  onMenu: () => void;
  /** Called after a destination is picked, like closing the drawer the rail is in. */
  onNavigate?: () => void;
  /** Lists the inbox, my issues and settings, which the top bar holds on wide screens. */
  hasAccountLinks?: boolean;
};

const showWhenExpanded =
  "ease-emphasized opacity-0 transition-opacity duration-100 group-data-expanded/rail-root:opacity-100 group-data-expanded/rail-root:duration-300";

/** Material 3's navigation rail: collapsed to icons, or expanded with labels beside them. */
export function NavigationRail({
  isExpanded,
  menuLabel,
  onMenu,
  onNavigate,
  hasAccountLinks = false,
}: TRailProps) {
  const { workspace, canEdit, isAdmin } = useWorkspace();
  const team = useWorkspaceTeam();
  const openNewIssue = useNewIssueStore((state) => state.open);
  const unread = useInboxUnreadCount();
  const params = { workspace: workspace.urlKey };
  const MenuToggleIcon = isExpanded ? ListIndentDecreaseIcon : MenuIcon;

  const destinations: TDestination[] = [
    {
      label: "Inbox",
      icon: InboxIcon,
      link: { to: "/$workspace/inbox", params },
      unread,
      isHidden: !hasAccountLinks,
    },
    {
      label: "My issues",
      icon: UserCircleIcon,
      link: { to: "/$workspace/my-issues", params },
      isHidden: !hasAccountLinks,
    },
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
    <nav
      aria-label="Workspace"
      data-expanded={isExpanded || undefined}
      className="group/rail-root bg-sidebar flex h-full w-full flex-col"
    >
      <div className="flex flex-col items-start gap-3 px-5 pt-3 pb-4">
        <SidebarToggleTooltip label={menuLabel} side="right">
          <Button
            variant="ghost"
            size="icon"
            aria-label={menuLabel}
            className="text-muted-foreground ml-2 size-10"
            onClick={onMenu}
          >
            <MenuToggleIcon className="size-5" />
          </Button>
        </SidebarToggleTooltip>
        {canEdit && (
          <SimpleTooltip content={isExpanded ? undefined : "New issue"} side="right">
            <Button
              variant="tonal"
              aria-label="New issue"
              className="dank:h-14 dank:rounded-[1rem] dank:px-4.5 dank:active:rounded-[0.75rem] flex items-center gap-0 text-base"
              onClick={() => {
                onNavigate?.();
                openNewIssue();
              }}
            >
              <SquarePenIcon className="size-5" />
              <span
                className={cn(
                  showWhenExpanded,
                  "max-w-0 overflow-hidden whitespace-nowrap transition-[max-width,margin,opacity] group-data-expanded/rail-root:ml-3 group-data-expanded/rail-root:max-w-40",
                )}
              >
                New issue
              </span>
            </Button>
          </SimpleTooltip>
        )}
      </div>
      <ScrollArea className="min-h-0 w-full flex-1">
        <div className="flex w-full flex-col gap-1">
          {destinations
            .filter((destination) => !destination.isHidden)
            .map((destination) => (
              <RailLink key={destination.label} {...destination} onNavigate={onNavigate} />
            ))}
          <MoreMenu isExpanded={isExpanded} onNavigate={onNavigate} />
          {isExpanded && (
            <div className="animate-in fade-in-0 flex flex-col px-3 pt-3 pb-2 duration-300">
              <SidebarFavorites onNavigate={onNavigate} />
            </div>
          )}
        </div>
      </ScrollArea>
      {hasAccountLinks && (
        <div className="flex w-full flex-col gap-1 pt-2 pb-3">
          <RailLink
            label="Settings"
            icon={SettingsIcon}
            onNavigate={onNavigate}
            link={{
              to: isAdmin ? "/$workspace/settings" : "/$workspace/settings/account",
              params,
            }}
          />
        </div>
      )}
    </nav>
  );
}

const railItemClassName =
  "group/rail text-muted-foreground has-hover:hover:text-foreground focus-visible:text-foreground data-[status=active]:text-secondary-foreground data-popup-open:text-foreground relative flex h-14 w-full items-start px-5 font-medium outline-hidden select-none";

function RailIndicator({ children }: { children: ReactNode }) {
  return (
    <span className="group-has-hover/rail:group-hover/rail:bg-accent group-focus-visible/rail:ring-primary group-data-[status=active]/rail:bg-secondary group-data-popup-open/rail:bg-accent ease-emphasized mt-0.5 flex h-8 w-full items-center overflow-hidden rounded-full pl-4.5 transition-[height,margin,background-color] duration-300 group-focus-visible/rail:ring-2 group-data-expanded/rail-root:mt-0 group-data-expanded/rail-root:h-14">
      {children}
    </span>
  );
}

function RailIcon({ icon: Icon, isActive }: { icon: LucideIcon; isActive?: boolean }) {
  return (
    <Icon
      data-active={isActive || undefined}
      className="size-5 shrink-0 data-active:fill-current"
    />
  );
}

function RailLabels({ label }: { label: string }) {
  return (
    <>
      <span className={cn(showWhenExpanded, "ml-3 text-sm whitespace-nowrap")}>{label}</span>
      <span
        aria-hidden
        className="ease-emphasized pointer-events-none absolute top-9.5 left-0 w-24 truncate px-1 text-center text-xs transition-opacity delay-100 duration-200 group-data-expanded/rail-root:opacity-0 group-data-expanded/rail-root:delay-0 group-data-expanded/rail-root:duration-100"
      >
        {label}
      </span>
    </>
  );
}

type TRailLinkProps = Omit<TDestination, "isHidden"> & { onNavigate?: () => void };

function RailLink({ label, icon, link, unread = 0, onNavigate }: TRailLinkProps) {
  return (
    <Link
      {...link}
      activeOptions={{ includeSearch: false }}
      className={railItemClassName}
      onClick={onNavigate}
    >
      {({ isActive }) => (
        <RailIndicator>
          <RailIcon icon={icon} isActive={isActive} />
          <RailLabels label={label} />
          {unread > 0 && <RailBadge count={unread} />}
        </RailIndicator>
      )}
    </Link>
  );
}

function RailButton({
  label,
  icon,
  ...props
}: ComponentProps<typeof Button> & { label: string; icon: LucideIcon }) {
  return (
    <Button
      variant="ghost"
      forceMinSize={false}
      {...props}
      className={`${railItemClassName} dank:h-14 dank:px-5 dank:active:rounded-none h-14 gap-0 rounded-none px-5 active:rounded-none active:bg-transparent has-hover:hover:bg-transparent`}
    >
      <RailIndicator>
        <RailIcon icon={icon} />
        <RailLabels label={label} />
      </RailIndicator>
    </Button>
  );
}

function RailBadge({ count }: { count: number }) {
  return (
    <span
      aria-label={`${count} unread`}
      className="bg-destructive text-destructive-foreground mr-5 ml-auto flex h-5 min-w-5 items-center justify-center rounded-full px-1 text-xs leading-none tabular-nums"
    >
      {count > 99 ? "99+" : count}
    </span>
  );
}

function MoreMenu({ isExpanded, onNavigate }: { isExpanded: boolean; onNavigate?: () => void }) {
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
      <DropdownMenuContent side={isExpanded ? "bottom" : "right"} className="w-56">
        <DropdownMenuGroup>
          {pages
            .filter((page) => !page.isHidden)
            .map((page) => (
              <DropdownMenuItem
                key={page.label}
                onClick={() => {
                  onNavigate?.();
                  void navigate(page.link);
                }}
              >
                <page.icon className="size-4" />
                {page.label}
              </DropdownMenuItem>
            ))}
        </DropdownMenuGroup>
        <DropdownMenuSeparator />
        <DropdownMenuGroup>
          <DropdownMenuItem
            onClick={() => {
              onNavigate?.();
              openShortcuts(true);
            }}
          >
            <KeyboardIcon className="size-4" />
            Keyboard shortcuts
          </DropdownMenuItem>
        </DropdownMenuGroup>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
