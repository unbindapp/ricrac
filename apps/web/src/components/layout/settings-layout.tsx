import { type LinkProps, useLocation } from "@tanstack/react-router";
import { ReactNode, useEffect, useRef } from "react";
import type { LucideIcon } from "lucide-react";

import { SidebarButton } from "@/components/layout/sidebar-button";
import { LinkButton } from "@/components/ui/button";

export type TSettingsTab = {
  label: string;
  icon: LucideIcon;
  link: LinkProps;
  exact?: boolean;
};

export type TSettingsTabGroup = {
  title: string;
  tabs: TSettingsTab[];
};

type TProps = {
  title: string;
  groups: TSettingsTabGroup[];
  children: ReactNode;
};

export function SettingsLayout({ title, groups, children }: TProps) {
  const navRef = useRef<HTMLElement>(null);
  const pathname = useLocation({ select: (location) => location.pathname });

  useEffect(() => {
    const activeTab = navRef.current?.querySelector("[data-status=active]");
    activeTab?.scrollIntoView({ block: "nearest", inline: "nearest" });
  }, [pathname]);

  return (
    <div className="flex min-h-0 w-full flex-1 flex-col">
      <header
        data-mobile-bar
        className="flex h-11 shrink-0 items-center gap-2 border-b px-4 md:h-auto md:border-0 md:px-8 md:pt-7 md:pb-3"
      >
        <SidebarButton />
        <h1 className="min-w-0 truncate text-sm font-medium md:text-2xl md:leading-tight md:font-semibold">
          {title}
        </h1>
      </header>
      <div className="flex min-h-0 flex-1 flex-col md:flex-row md:gap-4 md:pl-5">
        <nav
          ref={navRef}
          aria-label={title}
          className="scrollbar-hidden flex shrink-0 overflow-x-auto border-b px-2 md:w-52 md:flex-col md:gap-4 md:overflow-y-auto md:border-0 md:px-0 md:pb-8"
        >
          {groups.map((group) => (
            <div key={group.title} className="flex md:flex-col">
              <p className="text-muted-foreground hidden h-7 items-center px-3 text-xs font-medium md:flex">
                {group.title}
              </p>
              {group.tabs.map((tab) => (
                <SettingsTabLink key={tab.label} tab={tab} />
              ))}
            </div>
          ))}
        </nav>
        <div className="flex min-h-0 min-w-0 flex-1 flex-col overflow-y-auto">
          <div className="flex w-full max-w-3xl flex-col px-4 pt-4 pb-16 md:pt-1 md:pr-8 md:pl-0">
            {children}
          </div>
        </div>
      </div>
    </div>
  );
}

function SettingsTabLink({ tab }: { tab: TSettingsTab }) {
  return (
    <LinkButton
      {...tab.link}
      activeOptions={{ exact: tab.exact ?? false, includeSearch: false }}
      variant="ghost"
      size="sm"
      forceMinSize={false}
      className="text-muted-foreground data-[status=active]:text-foreground before:bg-foreground relative h-10 shrink-0 justify-start gap-2.5 px-3 font-medium before:absolute before:rounded-full before:opacity-0 before:content-[''] data-[status=active]:before:opacity-100 max-md:before:inset-x-3 max-md:before:bottom-0 max-md:before:h-0.5 md:h-8 md:before:inset-y-1.5 md:before:left-0 md:before:w-0.5"
    >
      <tab.icon className="size-4" />
      <span className="truncate">{tab.label}</span>
    </LinkButton>
  );
}
