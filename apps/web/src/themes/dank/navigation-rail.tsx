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
  PencilIcon,
  RefreshCwIcon,
  RocketIcon,
  SettingsIcon,
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
      data-slot="navigation-rail"
      data-expanded={isExpanded || undefined}
      className={cn(
        "group/rail-root flex h-full flex-col pt-11",
        isModal ? "bg-m3-surface-container w-full" : "bg-m3-surface w-(--rail-width)",
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
          <span className="relative block h-14 w-max">
            <span aria-hidden className="type-title-medium invisible block pr-4 pl-12">
              New issue
            </span>
            <SimpleTooltip content={isExpanded ? undefined : "New issue"} side="right">
              <Button
                variant="tonal"
                aria-label="New issue"
                forceMinSize={false}
                className="bg-m3-primary-container text-m3-on-primary-container shadow-elevation-1 has-hover:hover:shadow-elevation-2 absolute inset-y-0 left-0 block h-full w-(--rail-fab-width) overflow-hidden rounded-lg p-0 active:rounded-lg [&>svg]:size-6"
                onClick={() => {
                  onNavigate?.();
                  openNewIssue();
                }}
              >
                <PencilIcon className="absolute top-4 left-4" />
                <span className="type-title-medium ease-spring-fast-effects absolute top-4 left-12 whitespace-nowrap opacity-0 transition-opacity duration-(--spring-fast-effects-duration) group-data-expanded/rail-root:opacity-100">
                  New issue
                </span>
              </Button>
            </SimpleTooltip>
          </span>
        )}
      </div>
      <ScrollArea className="min-h-0 w-full flex-1">
        <div className="flex w-full flex-col gap-(--rail-item-gap) overflow-x-clip">
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
        <div className="flex w-full flex-col overflow-x-clip pt-2 pb-3">
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

const railItemClassName =
  "group/rail relative block h-(--rail-item-height) w-full shrink-0 outline-hidden select-none";

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
        className="ease-spring-default-spatial absolute top-(--rail-content-top) left-5 block h-(--rail-indicator-height) w-max max-w-64 pr-4 pl-12 text-left transition-[--rail-selected] duration-(--spring-default-spatial-duration) [--rail-selected:0] data-active:[--rail-selected:1]"
      >
        <span className="bg-m3-secondary-container absolute inset-y-0 left-0 w-[calc(var(--rail-indicator-width)*var(--rail-selected))] rounded-full opacity-(--rail-selected)" />
        <span
          className={cn(
            "group-focus-visible/rail:outline-m3-secondary absolute inset-y-0 left-0 w-(--rail-indicator-width) rounded-full group-focus-visible/rail:outline-3 group-focus-visible/rail:outline-offset-2 group-data-popup-open/rail:after:opacity-10",
            "after:bg-m3-on-secondary-container after:pointer-events-none after:absolute after:inset-0 after:rounded-full after:opacity-0 after:transition-opacity group-focus-visible/rail:after:opacity-10 group-active/rail:after:opacity-10 has-hover:group-hover/rail:after:opacity-8",
          )}
        />
        <span
          data-active={isActive || undefined}
          className="text-m3-on-surface-variant data-active:text-m3-on-secondary-container absolute top-(--rail-icon-top) left-4 flex"
        >
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
          data-active={isActive || undefined}
          className="type-label-large text-m3-on-surface-variant data-active:text-m3-on-secondary-container relative top-(--rail-start-label-top) left-(--rail-start-label-left) flex opacity-(--rail-start-label-opacity) transition-colors"
        >
          <span className="truncate">{label}</span>
          {unread > 0 && (
            <span aria-hidden className="ml-3 shrink-0 tabular-nums">
              {unread > 999 ? "999+" : unread}
            </span>
          )}
        </span>
      </span>
      <span
        aria-hidden
        data-active={isActive || undefined}
        className="type-label-medium text-m3-on-surface-variant data-active:text-m3-secondary absolute top-(--rail-top-label-top) left-0 w-24 truncate text-center opacity-(--rail-top-label-opacity) transition-colors"
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
        "rounded-none p-0 after:hidden active:rounded-none active:bg-transparent has-hover:hover:bg-transparent",
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
