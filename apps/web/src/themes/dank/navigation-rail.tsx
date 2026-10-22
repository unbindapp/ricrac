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
};

/** Material 3's navigation rail: collapsed to icons, or expanded with labels beside them. */
export function NavigationRail({ isExpanded, menuLabel, onMenu, onNavigate }: TRailProps) {
  const { workspace, canEdit, isAdmin } = useWorkspace();
  const team = useWorkspaceTeam();
  const openNewIssue = useNewIssueStore((state) => state.open);
  const unread = useInboxUnreadCount();
  const params = { workspace: workspace.urlKey };
  const MenuToggleIcon = isExpanded ? ListIndentDecreaseIcon : MenuIcon;

  const destinations: TDestination[] = [
    { label: "Inbox", icon: InboxIcon, link: { to: "/$workspace/inbox", params }, unread },
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
    <nav
      aria-label="Workspace"
      data-expanded={isExpanded || undefined}
      className="group/rail-root bg-sidebar flex h-full w-24 flex-col data-expanded:w-full"
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
              size={isExpanded ? "default" : "icon"}
              aria-label={isExpanded ? undefined : "New issue"}
              className={cn(
                "dank:rounded-[1rem] dank:active:rounded-[0.75rem] size-14",
                isExpanded && "dank:h-14 dank:pr-5 dank:pl-4.5 w-auto gap-3 text-base",
              )}
              onClick={() => {
                onNavigate?.();
                openNewIssue();
              }}
            >
              <SquarePenIcon className="size-5" />
              {isExpanded && "New issue"}
            </Button>
          </SimpleTooltip>
        )}
      </div>
      <ScrollArea className="min-h-0 w-full flex-1">
        <div className="flex w-full flex-col gap-1">
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
          {isExpanded && (
            <div className="flex flex-col px-3 pt-3 pb-2">
              <SidebarFavorites onNavigate={onNavigate} />
            </div>
          )}
        </div>
      </ScrollArea>
      <div className="flex w-full flex-col gap-1 pt-2 pb-3">
        <RailLink
          label="Settings"
          icon={SettingsIcon}
          isExpanded={isExpanded}
          onNavigate={onNavigate}
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
  "group/rail text-muted-foreground has-hover:hover:text-foreground focus-visible:text-foreground data-[status=active]:text-secondary-foreground data-popup-open:text-foreground flex h-14 w-full flex-col items-center justify-center gap-1 text-xs font-medium outline-hidden select-none group-data-expanded/rail-root:flex-row group-data-expanded/rail-root:justify-start";

function RailIndicator({ children }: { children: ReactNode }) {
  return (
    <span className="group-has-hover/rail:group-hover/rail:bg-accent group-focus-visible/rail:ring-primary group-data-[status=active]/rail:bg-secondary group-data-popup-open/rail:bg-accent relative flex h-8 w-14 shrink-0 items-center justify-center rounded-full transition-colors group-focus-visible/rail:ring-2 group-data-expanded/rail-root:mx-5 group-data-expanded/rail-root:h-14 group-data-expanded/rail-root:w-auto group-data-expanded/rail-root:flex-1 group-data-expanded/rail-root:justify-start group-data-expanded/rail-root:gap-3 group-data-expanded/rail-root:pr-5 group-data-expanded/rail-root:pl-4.5">
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

function RailLabel({ label, isExpanded }: { label: string; isExpanded: boolean }) {
  if (isExpanded) return <span className="min-w-0 flex-1 truncate text-left text-sm">{label}</span>;
  return <span className="max-w-full truncate px-1">{label}</span>;
}

type TRailLinkProps = TDestination & { isExpanded: boolean; onNavigate?: () => void };

function RailLink({ label, icon, link, unread = 0, isExpanded, onNavigate }: TRailLinkProps) {
  return (
    <Link
      {...link}
      activeOptions={{ includeSearch: false }}
      className={railItemClassName}
      onClick={onNavigate}
    >
      {({ isActive }) => (
        <>
          <RailIndicator>
            <RailIcon icon={icon} isActive={isActive} />
            {isExpanded && <RailLabel label={label} isExpanded />}
            {unread > 0 && <RailBadge count={unread} isExpanded={isExpanded} />}
          </RailIndicator>
          {!isExpanded && <RailLabel label={label} isExpanded={false} />}
        </>
      )}
    </Link>
  );
}

function RailButton({
  label,
  icon,
  isExpanded,
  ...props
}: ComponentProps<typeof Button> & { label: string; icon: LucideIcon; isExpanded: boolean }) {
  return (
    <Button
      variant="ghost"
      forceMinSize={false}
      {...props}
      className={`${railItemClassName} dank:h-14 dank:px-0 dank:active:rounded-none h-14 rounded-none px-0 active:rounded-none active:bg-transparent has-hover:hover:bg-transparent`}
    >
      <RailIndicator>
        <RailIcon icon={icon} />
        {isExpanded && <RailLabel label={label} isExpanded />}
      </RailIndicator>
      {!isExpanded && <RailLabel label={label} isExpanded={false} />}
    </Button>
  );
}

function RailBadge({ count, isExpanded }: { count: number; isExpanded: boolean }) {
  return (
    <span
      aria-label={`${count} unread`}
      data-expanded={isExpanded || undefined}
      className="bg-destructive text-destructive-foreground absolute top-0 right-2 flex h-4 min-w-4 items-center justify-center rounded-full px-1 text-xs leading-none tabular-nums data-expanded:static data-expanded:h-5 data-expanded:min-w-5"
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
      <DropdownMenuTrigger
        render={<RailButton label="More" icon={EllipsisIcon} isExpanded={isExpanded} />}
      />
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
