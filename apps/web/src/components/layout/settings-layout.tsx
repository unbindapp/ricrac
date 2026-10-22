import { Link, type LinkProps, useLocation, useMatchRoute } from "@tanstack/react-router";
import { ReactNode, useState } from "react";

import { ArrowLeftIcon, TIcon } from "@/components/icons/symbols.gen";
import { Button } from "@/components/ui/button";
import { cn } from "@/components/ui/utils";

export type TSettingsTab = {
  label: string;
  description: string;
  icon: TIcon;
  link: LinkProps;
  exact?: boolean;
  destructive?: boolean;
};

type TProps = {
  title: string;
  switcher: ReactNode;
  groups: TSettingsTab[][];
  children: ReactNode;
};

const groupBadges = [
  "bg-primary text-primary-foreground",
  "bg-tertiary text-tertiary-foreground",
  "bg-success text-success-foreground",
  "bg-process text-process-foreground",
  "bg-wait text-wait-foreground",
];

export function SettingsLayout({ title, switcher, groups, children }: TProps) {
  const pathname = useLocation({ select: (location) => location.pathname });
  const matchRoute = useMatchRoute();
  const firstTab = groups[0]?.[0];
  const [isListShown, setIsListShown] = useState(() =>
    firstTab ? Boolean(matchRoute(firstTab.link)) : false,
  );

  return (
    <div
      data-view={isListShown ? "list" : "page"}
      className="group/settings flex min-h-0 w-full flex-1"
    >
      <aside className="scrollbar-hidden flex min-h-0 w-full flex-col gap-3 overflow-y-auto p-3 group-data-[view=page]/settings:max-md:hidden md:w-[clamp(260px,36%,390px)] md:flex-none">
        {switcher}
        <nav aria-label={title} className="flex flex-col gap-2.5">
          {groups.map((tabs, index) => (
            <div key={tabs[0]?.label ?? index} className="flex flex-col gap-[3px]">
              {tabs.map((tab) => (
                <SettingsTabLink
                  key={tab.label}
                  tab={tab}
                  badge={groupBadges[index % groupBadges.length]}
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
