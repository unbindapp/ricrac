import { Link, useLocation, useMatchRoute, useNavigate } from "@tanstack/react-router";
import { useState } from "react";

import { ArrowLeftIcon } from "@/components/icons";
import type { TSettingsTab, TSettingsTabGroup } from "@/components/layout/settings-layout";
import { Button } from "@/components/ui/button";
import { cn } from "@/components/ui/utils";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { useSettingsReturnHref } from "@/lib/stores/settings-return";
import { SettingsWindow } from "@/themes/dank/settings-window";
import type { TSettingsFrameProps } from "@/themes/types";

const sectionBadges = [
  "bg-primary text-primary-foreground",
  "bg-tertiary text-tertiary-foreground",
  "bg-success text-success-foreground",
  "bg-process text-process-foreground",
  "bg-wait text-wait-foreground",
];

/** Settings in a window over the page; closing it goes back to where it opened. */
export function DankSettingsFrame({ groups, children }: TSettingsFrameProps) {
  const { workspace } = useWorkspace();
  const returnHref = useSettingsReturnHref(workspace.urlKey);
  const navigate = useNavigate();

  const close = () => {
    if (returnHref) {
      void navigate({ href: returnHref });
      return;
    }
    void navigate({ to: "/$workspace", params: { workspace: workspace.urlKey } });
  };

  return (
    <SettingsWindow onClose={close}>
      <SettingsList groups={groups}>{children}</SettingsList>
    </SettingsWindow>
  );
}

function useIsTabActive() {
  const matchRoute = useMatchRoute();
  return (tab: TSettingsTab) => Boolean(matchRoute({ ...tab.link, fuzzy: !tab.exact }));
}

function sectionsOf(tabs: TSettingsTab[]) {
  return tabs.reduce<TSettingsTab[][]>((sections, tab) => {
    const last = sections.at(-1);
    if (last && last[0].section === tab.section) last.push(tab);
    else sections.push([tab]);
    return sections;
  }, []);
}

function SettingsList({ groups, children }: TSettingsFrameProps) {
  const pathname = useLocation({ select: (location) => location.pathname });
  const isTabActive = useIsTabActive();
  const group = groups.find((item) => item.tabs.some(isTabActive)) ?? groups[0];
  const firstTab = group?.tabs[0];
  const [isListShown, setIsListShown] = useState(() =>
    firstTab ? isTabActive({ ...firstTab, exact: true }) : false,
  );

  return (
    <div
      data-view={isListShown ? "list" : "page"}
      className="group/settings flex min-h-0 w-full flex-1"
    >
      <aside className="scrollbar-hidden flex min-h-0 w-full flex-col gap-3 overflow-y-auto p-3 group-data-[view=page]/settings:max-md:hidden md:w-[clamp(260px,36%,390px)] md:flex-none">
        {groups.length > 1 && <GroupSwitcher groups={groups} current={group} />}
        <nav aria-label={group?.title} className="flex flex-col gap-2.5">
          {sectionsOf(group?.tabs ?? []).map((tabs, index) => (
            <div key={tabs[0].label} className="flex flex-col gap-[3px]">
              {tabs.map((tab) => (
                <SettingsTabLink
                  key={tab.label}
                  tab={tab}
                  badge={sectionBadges[index % sectionBadges.length]}
                  onSelect={() => setIsListShown(false)}
                />
              ))}
            </div>
          ))}
        </nav>
      </aside>
      <main
        key={pathname}
        className="bg-surface-container flex min-h-0 min-w-0 flex-1 flex-col overflow-y-auto group-data-[view=list]/settings:max-md:hidden"
      >
        <div className="animate-in fade-in-0 slide-in-from-right-8 ease-emphasized mx-auto flex w-full max-w-3xl flex-col gap-2 px-4 pt-4 pb-16 duration-300 motion-reduce:animate-none md:px-8 md:pt-6">
          <Button
            variant="ghost"
            size="icon"
            aria-label="Back to settings"
            className="-ml-2 size-10 md:hidden"
            onClick={() => setIsListShown(true)}
          >
            <ArrowLeftIcon className="size-5" />
          </Button>
          {children}
        </div>
      </main>
    </div>
  );
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
      className="bg-surface-container-highest grid h-14 shrink-0 auto-cols-fr grid-flow-col gap-1 rounded-full p-1"
    >
      {groups.map((group) => (
        <Link
          key={group.title}
          {...group.tabs[0]?.link}
          aria-current={group === current || undefined}
          data-current={group === current || undefined}
          className="text-muted-foreground has-hover:hover:bg-surface-container-high has-hover:hover:text-foreground data-current:bg-secondary data-current:text-secondary-foreground focus-visible:outline-primary ease-emphasized flex items-center justify-center gap-2 rounded-full text-base font-medium transition-colors duration-200 focus-visible:outline-2 focus-visible:-outline-offset-2"
        >
          <group.icon className="size-5" />
          {group.title}
        </Link>
      ))}
    </nav>
  );
}

type TTabLinkProps = {
  tab: TSettingsTab;
  badge: string;
  onSelect: () => void;
};

function SettingsTabLink({ tab, badge, onSelect }: TTabLinkProps) {
  return (
    <Link
      {...tab.link}
      activeOptions={{ exact: tab.exact ?? false, includeSearch: false }}
      onClick={onSelect}
      className="group/tab bg-surface-container text-foreground has-hover:hover:bg-surface-container-high focus-visible:outline-primary data-[status=active]:bg-secondary data-[status=active]:text-secondary-foreground ease-emphasized flex items-center gap-4 rounded-[0.375rem] px-5 py-3 transition-[background-color,border-radius] duration-200 first:rounded-t-[1.5rem] last:rounded-b-[1.5rem] focus-visible:outline-2 focus-visible:-outline-offset-2 data-[status=active]:rounded-[1.5rem]"
    >
      <span
        className={cn(
          "flex size-12 shrink-0 items-center justify-center rounded-full [&_svg]:size-6",
          tab.destructive ? "bg-destructive text-destructive-foreground" : badge,
        )}
      >
        <tab.icon />
      </span>
      <span className="flex min-w-0 flex-col gap-0.5">
        <span className="truncate text-base leading-tight font-medium">{tab.label}</span>
        <span className="text-muted-foreground group-data-[status=active]/tab:text-secondary-foreground truncate text-sm">
          {tab.description}
        </span>
      </span>
    </Link>
  );
}
