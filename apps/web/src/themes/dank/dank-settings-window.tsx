import { Link, useNavigate, useRouter, useSearch } from "@tanstack/react-router";
import { Suspense, useEffect, useState } from "react";

import { ArrowLeftIcon } from "@/components/icons";
import { ListSkeleton } from "@/components/layout/list-skeleton";
import type { TSettingsTab, TSettingsTabGroup } from "@/components/layout/settings-layout";
import { SettingsGate } from "@/components/settings/settings-gate";
import { useSettingsGroups } from "@/components/settings/settings-groups";
import { Button } from "@/components/ui/button";
import { cn } from "@/components/ui/utils";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { SettingsWindow } from "@/themes/dank/settings-window";

const sectionBadges = [
  "bg-primary text-primary-foreground",
  "bg-tertiary text-tertiary-foreground",
  "bg-success text-success-foreground",
  "bg-process text-process-foreground",
  "bg-wait text-wait-foreground",
];

/** Settings in a window over the page, which stays mounted under it while it is open. */
export function DankSettingsWindow() {
  const settingsPath = useSearch({
    from: "/$workspace/_workspace",
    select: (search) => search.settings,
  });
  const [shownPath, setShownPath] = useState(settingsPath);
  if (settingsPath && settingsPath !== shownPath) setShownPath(settingsPath);
  const navigate = useNavigate();

  const close = () =>
    void navigate({ to: ".", search: (prev) => ({ ...prev, settings: undefined }) });

  return (
    <SettingsWindow isOpen={Boolean(settingsPath)} onClose={close}>
      {shownPath && <SettingsList path={shownPath} />}
    </SettingsWindow>
  );
}

function settingsPathOf(tab: TSettingsTab) {
  return String(tab.link.to).replace("/$workspace", "");
}

function isTabActive(tab: TSettingsTab, path: string) {
  const tabPath = settingsPathOf(tab);
  return path === tabPath || (!tab.exact && path.startsWith(`${tabPath}/`));
}

function sectionsOf(tabs: TSettingsTab[]) {
  return tabs.reduce<TSettingsTab[][]>((sections, tab) => {
    const last = sections.at(-1);
    if (last && last[0].section === tab.section) last.push(tab);
    else sections.push([tab]);
    return sections;
  }, []);
}

function SettingsList({ path }: { path: string }) {
  const router = useRouter();
  const groups = useSettingsGroups();
  useEffect(() => {
    for (const tab of groups.flatMap((group) => group.tabs)) {
      router.preloadRoute(tab.link).catch(() => undefined);
    }
  }, [router, groups]);
  const group = groups.find((item) => item.tabs.some((tab) => isTabActive(tab, path))) ?? groups[0];
  const firstTab = group?.tabs[0];
  const [isListShown, setIsListShown] = useState(() =>
    firstTab ? path === settingsPathOf(firstTab) : false,
  );
  const [openedPath] = useState(path);

  return (
    <div
      data-view={isListShown ? "list" : "page"}
      className="group/settings flex min-h-0 w-full flex-1 gap-6"
    >
      <aside className="scrollbar-hidden flex min-h-0 w-full flex-col gap-3 overflow-y-auto max-md:p-4 group-data-[view=page]/settings:max-md:hidden md:w-[clamp(260px,36%,390px)] md:flex-none">
        {groups.length > 1 && <GroupSwitcher groups={groups} current={group} />}
        <nav aria-label={group?.title} className="flex flex-col gap-2.5">
          {sectionsOf(group?.tabs ?? []).map((tabs, index) => (
            <div key={tabs[0].label} className="flex flex-col gap-0.5">
              {tabs.map((tab) => (
                <SettingsTabLink
                  key={tab.label}
                  tab={tab}
                  isActive={isTabActive(tab, path)}
                  badge={sectionBadges[index % sectionBadges.length]}
                  onSelect={() => setIsListShown(false)}
                />
              ))}
            </div>
          ))}
        </nav>
      </aside>
      <main
        key={path}
        className="bg-surface-container flex min-h-0 min-w-0 flex-1 flex-col overflow-y-auto group-data-[view=list]/settings:max-md:hidden md:rounded-lg"
      >
        <div
          data-switched={path !== openedPath || undefined}
          className="data-switched:animate-in data-switched:fade-in-0 data-switched:slide-in-from-right-8 ease-emphasized mx-auto flex w-full max-w-3xl flex-col gap-2 px-4 pt-4 pb-16 duration-300 motion-reduce:animate-none md:px-8 md:pt-6"
        >
          <Button
            variant="ghost"
            size="icon"
            aria-label="Back to settings"
            className="-ml-2 size-10 md:hidden"
            onClick={() => setIsListShown(true)}
          >
            <ArrowLeftIcon className="size-5" />
          </Button>
          <SettingsPage path={path} />
        </div>
      </main>
    </div>
  );
}

const WORKSPACE_SETTINGS_ROUTE = "/$workspace/_workspace/settings/_workspace-settings";
const ADMIN_SETTINGS_ROUTE = "/$workspace/_workspace/settings/_workspace-settings/_admin";

/** The settings route's page for a path, rendered here so the page under the window stays. */
function SettingsPage({ path }: { path: string }) {
  const router = useRouter();
  const { workspace } = useWorkspace();
  const [routes, , found] = router.getMatchedRoutes(`/${workspace.urlKey}${path}`);
  const Page = found?.options.component;
  if (!Page) return null;
  const ids = routes.map((route) => route.id);
  const page = (
    <Suspense fallback={<ListSkeleton rows={4} />}>
      <Page />
    </Suspense>
  );
  if (ids.includes(ADMIN_SETTINGS_ROUTE))
    return <SettingsGate access="admins">{page}</SettingsGate>;
  if (ids.includes(WORKSPACE_SETTINGS_ROUTE)) {
    return <SettingsGate access="members">{page}</SettingsGate>;
  }
  return page;
}

function GroupSwitcher({
  groups,
  current,
}: {
  groups: TSettingsTabGroup[];
  current: TSettingsTabGroup | undefined;
}) {
  return (
    <nav
      aria-label="Settings sections"
      className="grid h-12 shrink-0 auto-cols-fr grid-flow-col shadow-[inset_0_-1px_0_var(--m3-outline-variant)]"
    >
      {groups.map((group) => (
        <Link
          key={group.title}
          {...group.tabs[0]?.link}
          aria-current={group === current || undefined}
          data-current={group === current || undefined}
          className="group/group-tab state-layer text-m3-on-surface-variant type-title-small has-hover:hover:text-m3-on-surface data-current:text-m3-primary has-hover:hover:data-current:text-m3-primary relative flex items-center justify-center px-4"
        >
          <span className="after:bg-m3-primary relative flex items-center gap-2 after:absolute after:inset-x-0 after:-bottom-3 after:h-[3px] after:rounded-[3px] after:opacity-0 group-data-current/group-tab:after:opacity-100">
            <group.icon className="size-6" />
            {group.title}
          </span>
        </Link>
      ))}
    </nav>
  );
}

type TTabLinkProps = {
  tab: TSettingsTab;
  isActive: boolean;
  badge: string;
  onSelect: () => void;
};

function SettingsTabLink({ tab, isActive, badge, onSelect }: TTabLinkProps) {
  return (
    <Link
      {...tab.link}
      data-current={isActive || undefined}
      data-focus-indicator="child"
      onClick={onSelect}
      className="group/tab state-layer bg-m3-surface text-m3-on-surface data-current:bg-m3-secondary-container data-current:text-m3-on-secondary-container focus-visible:outline-m3-secondary ease-spring-fast-spatial relative flex min-h-18 items-center gap-3 rounded-xs px-4 py-2.5 transition-[background-color,border-radius] duration-(--spring-fast-spatial-duration) first:rounded-t-lg last:rounded-b-lg focus-visible:rounded-lg focus-visible:outline-3 focus-visible:-outline-offset-3 focus-visible:outline-solid has-hover:hover:rounded-md data-current:rounded-lg has-hover:hover:data-current:rounded-lg"
    >
      <span
        className={cn(
          "flex size-10 shrink-0 items-center justify-center rounded-full [&_svg]:size-6",
          tab.destructive ? "bg-destructive text-destructive-foreground" : badge,
        )}
      >
        <tab.icon />
      </span>
      <span className="flex min-w-0 flex-col">
        <span className="type-body-large truncate">{tab.label}</span>
        <span className="type-body-medium text-m3-on-surface-variant group-data-current/tab:text-m3-on-secondary-container truncate">
          {tab.description}
        </span>
      </span>
    </Link>
  );
}
