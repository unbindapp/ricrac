import { Link, type LinkProps, useNavigate, useRouterState } from "@tanstack/react-router";
import { type ComponentProps, type ComponentType } from "react";

import { useCommandPanelStore } from "@/components/command-panel/command-panel-store";
import { SidebarFavorites } from "@/components/favorites/sidebar-favorites";
import {
  BoxIcon,
  BuildingIcon,
  CircleDotIcon,
  DiamondIcon,
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
  TargetIcon,
  type LucideIcon,
  TriangleIcon,
  UserCircleIcon,
} from "@/components/icons";
import { useInboxUnreadCount } from "@/components/inbox/inbox-unread-badge";
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
  /** Pages under another path that belong to the destination, like a forum's discussions. */
  alsoUnder?: string;
};

type TRailProps = {
  isExpanded: boolean;
  menuLabel: string;
  onMenu: () => void;
  /** Called after a destination is picked, like closing the drawer the rail is in. */
  onNavigate?: () => void;
  /** Over the page in a drawer, where it also switches workspaces. */
  isModal?: boolean;
};

/** The navigation rail: icons in pills, with their labels beside them while expanded. */
export function NavigationRail({
  isExpanded,
  menuLabel,
  onMenu,
  onNavigate,
  isModal = false,
}: TRailProps) {
  const { workspace, isAdmin } = useWorkspace();
  const team = useWorkspaceTeam();
  const unread = useInboxUnreadCount();
  const params = { workspace: workspace.urlKey };
  const MenuToggleIcon = isExpanded ? ListIndentDecreaseIcon : MenuIcon;

  const destinations: TDestination[] = [
    { label: "Inbox", icon: InboxIcon, link: { to: "/$workspace/inbox", params }, unread },
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
    {
      label: "Forum",
      icon: MessagesSquareIcon,
      link: { to: "/$workspace/forum", params },
      alsoUnder: `/${workspace.urlKey}/discussion/`,
    },
  ];

  return (
    <nav
      aria-label="Workspace"
      data-slot="navigation-rail"
      data-expanded={isExpanded || undefined}
      className={cn(
        "group/rail-root grid h-full shrink-0 grid-rows-[1fr_auto_1fr] overflow-x-hidden overflow-y-auto px-4 py-3",
        isModal
          ? "bg-m3-surface-container w-full"
          : "w-22 transition-[width] duration-250 ease-[ease] data-expanded:w-70",
      )}
    >
      <div className="flex flex-col items-start">
        <SidebarToggleTooltip label={menuLabel} side="right">
          <Button
            variant="ghost"
            size="icon"
            forceMinSize={false}
            aria-label={menuLabel}
            className="text-m3-on-surface has-hover:hover:bg-m3-surface-variant size-14 rounded-full after:hidden active:rounded-full"
            onClick={onMenu}
          >
            <MenuToggleIcon />
          </Button>
        </SidebarToggleTooltip>
      </div>
      <div className="row-start-2 flex flex-col gap-2">
        {destinations
          .filter((destination) => !destination.isHidden)
          .map((destination) => (
            <RailLink
              key={destination.label}
              {...destination}
              isExpanded={isExpanded}
              onNavigate={onNavigate}
            />
          ))}
        <MoreMenu isExpanded={isExpanded} onNavigate={onNavigate} />
        <RailLink
          label="Settings"
          icon={SettingsIcon}
          isExpanded={isExpanded}
          onNavigate={onNavigate}
          link={{ to: isAdmin ? "/$workspace/settings" : "/$workspace/settings/account", params }}
        />
      </div>
      <div className="flex min-w-0 flex-col justify-between gap-2 pt-3">
        {isExpanded && (
          <div className="animate-in fade-in-0 flex flex-col">
            <SidebarFavorites onNavigate={onNavigate} />
          </div>
        )}
        {isModal && (
          <WorkspaceMenu
            onNavigate={onNavigate}
            trigger={
              <RailButton
                label={workspace.name}
                icon={CurrentWorkspaceIcon}
                isExpanded
                aria-label="Switch workspace"
                className="mt-auto"
              />
            }
          />
        )}
      </div>
    </nav>
  );
}

const railItemClassName =
  "relative flex h-14 w-full shrink-0 items-center justify-start gap-3 rounded-full px-4 text-left text-[15px] leading-5 font-medium whitespace-nowrap transition-colors select-none";

type TRailIcon = ComponentType<{ className?: string }>;

function RailItemContent({
  icon: Icon,
  label,
  isExpanded,
  isActive = false,
  unread = 0,
}: {
  icon: TRailIcon;
  label: string;
  isExpanded: boolean;
  isActive?: boolean;
  unread?: number;
}) {
  const unreadLabel = unread > 999 ? "999+" : String(unread);
  return (
    <>
      <span className="relative flex shrink-0">
        <Icon data-active={isActive || undefined} className="size-6 data-active:fill-current" />
        {unread > 0 && !isExpanded && (
          <span
            aria-label={`${unread} unread`}
            className="bg-m3-error text-m3-on-error type-label-small absolute -top-1 left-3.5 flex h-4 min-w-4 items-center justify-center rounded-full px-1 tabular-nums"
          >
            {unreadLabel}
          </span>
        )}
      </span>
      {isExpanded && <span className="min-w-0 flex-1 truncate">{label}</span>}
      {isExpanded && unread > 0 && (
        <span aria-label={`${unread} unread`} className="shrink-0 tabular-nums">
          {unreadLabel}
        </span>
      )}
    </>
  );
}

type TRailLinkProps = Omit<TDestination, "isHidden"> & {
  isExpanded: boolean;
  onNavigate?: () => void;
};

function RailLink({
  label,
  icon,
  link,
  unread = 0,
  alsoUnder,
  isExpanded,
  onNavigate,
}: TRailLinkProps) {
  const pathname = useRouterState({ select: (state) => state.location.pathname });
  const isUnder = Boolean(alsoUnder && pathname.startsWith(alsoUnder));
  return (
    <SimpleTooltip content={isExpanded ? undefined : label} side="right">
      <Link
        {...link}
        aria-label={isExpanded ? undefined : label}
        aria-current={isUnder ? "page" : undefined}
        data-current={isUnder || undefined}
        activeOptions={{ includeSearch: false }}
        data-slot="rail-item"
        className={railItemClassName}
        onClick={onNavigate}
      >
        {({ isActive }) => (
          <RailItemContent
            icon={icon}
            label={label}
            isExpanded={isExpanded}
            isActive={isActive || isUnder}
            unread={unread}
          />
        )}
      </Link>
    </SimpleTooltip>
  );
}

function RailButton({
  label,
  icon,
  isExpanded,
  className,
  ...props
}: ComponentProps<typeof Button> & { label: string; icon: TRailIcon; isExpanded: boolean }) {
  return (
    <Button
      variant="ghost"
      forceMinSize={false}
      aria-label={isExpanded ? undefined : label}
      {...props}
      data-slot="rail-item"
      className={cn(railItemClassName, "after:hidden active:rounded-full", className)}
    >
      <RailItemContent icon={icon} label={label} isExpanded={isExpanded} />
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
    { label: "My issues", icon: UserCircleIcon, link: { to: "/$workspace/my-issues", params } },
    { label: "Milestones", icon: DiamondIcon, link: { to: "/$workspace/milestones", params } },
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
      <SimpleTooltip content={isExpanded ? undefined : "More"} side="right">
        <DropdownMenuTrigger
          render={<RailButton label="More" icon={EllipsisIcon} isExpanded={isExpanded} />}
        />
      </SimpleTooltip>
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
