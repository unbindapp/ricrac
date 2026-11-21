import { Link, type LinkProps, useRouterState } from "@tanstack/react-router";

import { SidebarFavorites } from "@/components/favorites/sidebar-favorites";
import {
  BadgeCheckIcon,
  CircleCheckBigIcon,
  FileTextIcon,
  FlagIcon,
  FolderIcon,
  KanbanIcon,
  type LucideIcon,
  MessagesSquareIcon,
  RefreshCwIcon,
  RocketIcon,
  Rows2Icon,
  TriangleIcon,
  UsersRoundIcon,
} from "@/components/icons";
import { useWorkspaceTeam } from "@/components/workspace/teams-provider";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { useHotkeyScope } from "@/lib/hotkeys/use-hotkey-scope";
import { useRememberSettingsReturn } from "@/lib/stores/settings-return";
import { SIDEBAR_HOTKEY, useSidebarStore } from "@/lib/stores/sidebar";

type TDestination = {
  label: string;
  icon: LucideIcon;
  link: LinkProps;
  isHidden?: boolean;
  /** Pages under another path that belong to the destination, like a forum's discussions. */
  alsoUnder?: string;
};

export const DUX_SIDEBAR_ID = "dux-sidebar";

/** The sidebar beside pages on wide screens; the app bar's menu button hides it. */
export function DuxNavigation() {
  const isHidden = useSidebarStore((state) => state.isCollapsed);
  const toggle = useSidebarStore((state) => state.toggle);
  useHotkeyScope([{ hotkey: SIDEBAR_HOTKEY, callback: toggle }]);
  useRememberSettingsReturn();
  return (
    <aside
      id={DUX_SIDEBAR_ID}
      data-slot="dux-sidebar"
      data-hidden={isHidden || undefined}
      inert={isHidden}
      className="max-md:hidden"
    >
      <SidebarNav />
    </aside>
  );
}

/** The sidebar's destinations in the drawer on narrow screens. */
export function DuxNavigationDrawer({ onNavigate }: { onNavigate: () => void }) {
  return (
    <div data-slot="dux-sidebar" data-drawer="">
      <SidebarNav onNavigate={onNavigate} />
    </div>
  );
}

function SidebarNav({ onNavigate }: { onNavigate?: () => void }) {
  const { workspace, canEdit } = useWorkspace();
  const team = useWorkspaceTeam();
  const params = { workspace: workspace.urlKey };

  const destinations: TDestination[] = [
    { label: "Issues", icon: CircleCheckBigIcon, link: { to: "/$workspace/issues", params } },
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
    { label: "Projects", icon: FolderIcon, link: { to: "/$workspace/projects", params } },
    { label: "Boards", icon: KanbanIcon, link: { to: "/$workspace/boards", params } },
    { label: "Docs", icon: FileTextIcon, link: { to: "/$workspace/documents", params } },
    {
      label: "Forum",
      icon: MessagesSquareIcon,
      link: { to: "/$workspace/forum", params },
      alsoUnder: `/${workspace.urlKey}/discussion/`,
    },
    {
      label: "Initiatives",
      icon: RocketIcon,
      link: { to: "/$workspace/initiatives", params },
      isHidden: !canEdit,
    },
    { label: "Milestones", icon: FlagIcon, link: { to: "/$workspace/milestones", params } },
    { label: "Releases", icon: BadgeCheckIcon, link: { to: "/$workspace/releases", params } },
    { label: "Views", icon: Rows2Icon, link: { to: "/$workspace/views", params } },
    {
      label: "Customers",
      icon: UsersRoundIcon,
      link: { to: "/$workspace/customers", params },
      isHidden: !canEdit,
    },
  ];

  return (
    <nav aria-label="Primary" data-slot="dux-nav">
      {destinations
        .filter((destination) => !destination.isHidden)
        .map((destination) => (
          <NavItem key={destination.label} {...destination} onNavigate={onNavigate} />
        ))}
      <SidebarFavorites onNavigate={onNavigate} />
    </nav>
  );
}

function NavItem({
  label,
  icon: Icon,
  link,
  alsoUnder,
  onNavigate,
}: TDestination & { onNavigate?: () => void }) {
  const pathname = useRouterState({ select: (state) => state.location.pathname });
  const isUnder = Boolean(alsoUnder && pathname.startsWith(alsoUnder));
  return (
    <Link
      {...link}
      aria-current={isUnder ? "page" : undefined}
      data-current={isUnder || undefined}
      activeOptions={{ includeSearch: false }}
      data-slot="dux-nav-item"
      onClick={onNavigate}
    >
      {({ isActive }) => (
        <>
          <Icon
            data-active={isActive || isUnder || undefined}
            className="data-active:fill-current"
          />
          <span className="min-w-0 flex-1 truncate">{label}</span>
        </>
      )}
    </Link>
  );
}
