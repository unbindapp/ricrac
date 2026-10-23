import { Link, type LinkProps, useNavigate } from "@tanstack/react-router";
import { type ComponentProps, type ComponentType } from "react";

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
import { WorkspaceIcon } from "@/components/workspace/workspace-icon";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { WorkspaceMenu } from "@/components/workspace/workspace-switcher";

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
  /** Over the page in a drawer, on the modal rail's container color. */
  isModal?: boolean;
};

const spatial =
  "ease-spring-default-spatial duration-(--spring-default-spatial-duration) [interpolate-size:allow-keywords]";

/** Material 3's navigation rail: collapsed to icons over labels, or expanded with labels beside them. */
export function NavigationRail({
  isExpanded,
  menuLabel,
  onMenu,
  onNavigate,
  hasAccountLinks = false,
  isModal = false,
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
      className={cn(
        "group/rail-root flex h-full w-full flex-col pt-11",
        isModal ? "bg-m3-surface-container" : "bg-m3-surface",
      )}
    >
      <div className="flex flex-col items-start gap-1 px-5 pb-10">
        <SidebarToggleTooltip label={menuLabel} side="right">
          <Button
            variant="ghost"
            size="icon"
            aria-label={menuLabel}
            className="text-m3-on-surface-variant mx-2"
            onClick={onMenu}
          >
            <MenuToggleIcon className="size-6" />
          </Button>
        </SidebarToggleTooltip>
        {canEdit && (
          <SimpleTooltip content={isExpanded ? undefined : "New issue"} side="right">
            <Button
              variant="tonal"
              aria-label="New issue"
              forceMinSize={false}
              className={cn(
                spatial,
                "bg-m3-primary-container text-m3-on-primary-container shadow-elevation-1 has-hover:hover:shadow-elevation-2 dank:h-14 dank:rounded-lg dank:px-4 dank:text-base dank:tracking-[0.009375rem] dank:active:rounded-lg flex items-center gap-0 transition-[box-shadow,padding] [&>svg]:size-6",
              )}
              onClick={() => {
                onNavigate?.();
                openNewIssue();
              }}
            >
              <SquarePenIcon className="size-6" />
              <span
                className={cn(
                  spatial,
                  "max-w-0 overflow-hidden whitespace-nowrap opacity-0 transition-[max-width,margin,opacity] group-data-expanded/rail-root:ml-2 group-data-expanded/rail-root:max-w-40 group-data-expanded/rail-root:opacity-100",
                )}
              >
                New issue
              </span>
            </Button>
          </SimpleTooltip>
        )}
      </div>
      <ScrollArea className="min-h-0 w-full flex-1">
        <div
          className={cn(
            spatial,
            "flex w-full flex-col gap-1 transition-[gap] group-data-expanded/rail-root:gap-0",
          )}
        >
          {destinations
            .filter((destination) => !destination.isHidden)
            .map((destination) => (
              <RailLink key={destination.label} {...destination} onNavigate={onNavigate} />
            ))}
          <MoreMenu isExpanded={isExpanded} onNavigate={onNavigate} />
          {isExpanded && (
            <div className="animate-in fade-in-0 ease-spring-default-effects flex flex-col px-3 pt-3 pb-2 duration-(--spring-default-effects-duration)">
              <SidebarFavorites onNavigate={onNavigate} />
            </div>
          )}
        </div>
      </ScrollArea>
      {hasAccountLinks && (
        <div className="flex w-full flex-col pt-2 pb-3">
          <WorkspaceMenu
            onNavigate={onNavigate}
            trigger={
              <RailButton
                label={workspace.name}
                icon={CurrentWorkspaceIcon}
                aria-label="Switch workspace"
              />
            }
          />
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

const railItemClassName = cn(
  spatial,
  "group/rail relative flex min-h-16 w-full flex-col items-start gap-1 px-5 py-1.5 outline-hidden select-none transition-[min-height,padding,gap] group-data-expanded/rail-root:min-h-14 group-data-expanded/rail-root:gap-0 group-data-expanded/rail-root:py-0",
);

type TRailIcon = ComponentType<{ className?: string }>;

function RailItemContent({
  icon: Icon,
  label,
  isActive = false,
  unread = 0,
}: {
  icon: TRailIcon;
  label: string;
  isActive?: boolean;
  unread?: number;
}) {
  return (
    <>
      <span
        data-active={isActive || undefined}
        className={cn(
          spatial,
          "text-m3-on-surface-variant data-active:bg-m3-secondary-container data-active:text-m3-on-secondary-container group-focus-visible/rail:outline-m3-secondary relative inline-flex h-8 min-w-14 items-center rounded-full px-4 transition-[height,background-color] group-focus-visible/rail:outline-3 group-focus-visible/rail:outline-offset-2 group-data-expanded/rail-root:h-14 group-data-popup-open/rail:after:opacity-10",
          "after:bg-m3-on-secondary-container after:pointer-events-none after:absolute after:inset-0 after:rounded-full after:opacity-0 after:transition-opacity group-focus-visible/rail:after:opacity-10 group-active/rail:after:opacity-10 has-hover:group-hover/rail:after:opacity-8",
        )}
      >
        <span className="relative flex shrink-0">
          <Icon data-active={isActive || undefined} className="size-6 data-active:fill-current" />
          {unread > 0 && (
            <span
              aria-label={`${unread} unread`}
              className="bg-m3-error text-m3-on-error absolute -top-0.5 left-3 flex h-4 min-w-4 items-center justify-center rounded-full px-1 text-[0.6875rem] leading-4 font-medium tracking-[0.03125rem] tabular-nums transition-opacity group-data-expanded/rail-root:opacity-0"
            >
              {unread > 999 ? "999+" : unread}
            </span>
          )}
        </span>
        <span
          className={cn(
            spatial,
            "max-w-0 overflow-hidden text-sm font-medium tracking-[0.00625rem] whitespace-nowrap opacity-0 transition-[max-width,margin,opacity] group-data-expanded/rail-root:ml-2 group-data-expanded/rail-root:max-w-48 group-data-expanded/rail-root:opacity-100",
          )}
        >
          {label}
        </span>
        {unread > 0 && (
          <span
            aria-hidden
            className="max-w-0 overflow-hidden text-sm font-medium tracking-[0.00625rem] tabular-nums opacity-0 transition-opacity group-data-expanded/rail-root:ml-3 group-data-expanded/rail-root:max-w-12 group-data-expanded/rail-root:opacity-100"
          >
            {unread > 999 ? "999+" : unread}
          </span>
        )}
      </span>
      <span
        aria-hidden
        data-active={isActive || undefined}
        className={cn(
          spatial,
          "text-m3-on-surface-variant data-active:text-m3-secondary -mx-5 h-4 w-24 truncate px-1 text-center text-xs leading-4 font-medium tracking-[0.03125rem] transition-[height,opacity] group-data-expanded/rail-root:h-0 group-data-expanded/rail-root:opacity-0",
        )}
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
      data-focus-indicator="child"
      activeOptions={{ includeSearch: false }}
      className={railItemClassName}
      onClick={onNavigate}
    >
      {({ isActive }) => (
        <RailItemContent icon={icon} label={label} isActive={isActive} unread={unread} />
      )}
    </Link>
  );
}

function RailButton({
  label,
  icon,
  ...props
}: ComponentProps<typeof Button> & { label: string; icon: TRailIcon }) {
  return (
    <Button
      variant="ghost"
      forceMinSize={false}
      data-focus-indicator="child"
      {...props}
      className={cn(
        railItemClassName,
        "dank:h-auto dank:px-5 dank:active:rounded-none h-auto rounded-none active:bg-transparent has-hover:hover:bg-transparent",
      )}
    >
      <RailItemContent icon={icon} label={label} />
    </Button>
  );
}

function CurrentWorkspaceIcon({ className }: { className?: string }) {
  const { workspace } = useWorkspace();
  return (
    <span className={cn(className, "flex items-center justify-center")}>
      <WorkspaceIcon name={workspace.name} logoUrl={workspace.logoUrl} />
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
