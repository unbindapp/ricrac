import { type LinkProps } from "@tanstack/react-router";
import { LucideIcon } from "lucide-react";
import { ReactNode } from "react";

import { SidebarButton } from "@/components/layout/sidebar-button";
import { PageContainer } from "@/components/layout/page-container";
import { LinkButton } from "@/components/ui/button";

export type TSettingsTab = {
  label: string;
  icon: LucideIcon;
  link: LinkProps;
  exact?: boolean;
};

type TProps = {
  title: string;
  tabs: TSettingsTab[];
  children: ReactNode;
};

export function SettingsLayout({ title, tabs, children }: TProps) {
  return (
    <div className="flex min-h-0 w-full flex-1 flex-col lg:flex-row">
      <div data-mobile-bar className="flex shrink-0 items-center border-b pl-4 md:pl-0 lg:contents">
        <SidebarButton />
        <nav
          aria-label={title}
          className="scrollbar-hidden flex min-w-0 flex-1 items-center gap-1 overflow-x-auto px-3 py-2 lg:w-72 lg:flex-none lg:flex-col lg:items-stretch lg:gap-0 lg:overflow-y-auto lg:px-3 lg:py-4"
        >
          <p className="text-primary hidden h-8 items-center px-3 text-sm font-medium lg:flex">
            {title}
          </p>
          {tabs.map((tab) => (
            <LinkButton
              key={tab.label}
              {...tab.link}
              activeOptions={{ exact: tab.exact ?? false, includeSearch: false }}
              variant="ghost"
              size="sm"
              forceMinSize={false}
              className="group/tab data-[status=active]:bg-secondary data-[status=active]:text-secondary-foreground lg:hit-gap-y lg:bg-surface-container-high lg:has-hover:hover:bg-surface-container-highest lg:data-[status=active]:bg-secondary h-8 shrink-0 justify-start gap-3 px-3 font-normal lg:h-12 lg:rounded-xs lg:px-2 lg:first-of-type:rounded-t-lg lg:last-of-type:rounded-b-lg lg:data-[status=active]:rounded-lg"
            >
              <span className="lg:bg-primary lg:text-primary-foreground group-data-[status=active]/tab:lg:bg-secondary-foreground group-data-[status=active]/tab:lg:text-secondary flex shrink-0 items-center justify-center rounded-full transition-colors lg:size-8 [&_svg]:size-4">
                <tab.icon />
              </span>
              <span className="truncate lg:font-medium">{tab.label}</span>
            </LinkButton>
          ))}
        </nav>
      </div>
      <PageContainer width="narrow">{children}</PageContainer>
    </div>
  );
}
