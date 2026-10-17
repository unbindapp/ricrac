import { ReactNode } from "react";

import { Breadcrumb, TBreadcrumbItem } from "@/components/layout/breadcrumb";

type TProps = {
  title: string;
  description?: string;
  breadcrumb?: TBreadcrumbItem[];
  actions?: ReactNode;
  children: ReactNode;
};

export function SettingsPage({ title, description, breadcrumb, actions, children }: TProps) {
  return (
    <div data-slot="settings-page" className="flex w-full flex-col gap-8">
      <header data-slot="page-header" className="flex w-full flex-col gap-2">
        {breadcrumb && breadcrumb.length > 0 && <Breadcrumb items={breadcrumb} />}
        <div className="flex min-h-8 w-full items-center gap-2">
          <h1 className="min-w-0 flex-1 truncate text-2xl leading-tight font-semibold">{title}</h1>
          {actions && <div className="flex shrink-0 items-center gap-2">{actions}</div>}
        </div>
        {description && <p className="text-muted-foreground text-sm">{description}</p>}
      </header>
      {children}
    </div>
  );
}
