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
          className="scrollbar-hidden flex min-w-0 flex-1 items-center overflow-x-auto px-3 py-2 lg:w-56 lg:flex-none lg:flex-col lg:items-stretch lg:overflow-y-auto lg:border-r lg:px-2 lg:py-4"
        >
          <p className="text-muted-foreground hidden h-7 items-center px-2 text-xs font-medium lg:flex">
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
              className="hit-gap-x lg:hit-gap-y data-[status=active]:bg-accent [&_svg]:text-muted-foreground h-7.5 shrink-0 justify-start px-2 font-normal lg:border-x-0"
            >
              <tab.icon className="size-4" />
              <span className="truncate">{tab.label}</span>
            </LinkButton>
          ))}
        </nav>
      </div>
      <PageContainer width="narrow">{children}</PageContainer>
    </div>
  );
}
