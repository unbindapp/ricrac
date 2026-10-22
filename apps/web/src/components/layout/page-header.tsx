import { ReactNode } from "react";
import { createPortal } from "react-dom";

import { Breadcrumb, TBreadcrumbItem } from "@/components/layout/breadcrumb";
import { usePageSlots } from "@/components/layout/page-container";
import { SidebarButton } from "@/components/layout/sidebar-button";
import { cn } from "@/components/ui/utils";

type TProps = {
  title: string;
  icon?: ReactNode;
  description?: string;
  breadcrumb?: TBreadcrumbItem[];
  actions?: ReactNode;
  toolbar?: ReactNode;
};

export function PageHeader({ title, icon, description, breadcrumb, actions, toolbar }: TProps) {
  return (
    <header data-slot="page-header" className="flex w-full flex-col gap-3">
      <PageBar>
        <div className="min-w-0 flex-1">
          <Breadcrumb
            items={[
              ...(breadcrumb ?? []),
              { label: title, icon: icon && <TitleIcon icon={icon} /> },
            ]}
          />
        </div>
        {actions && <div className="flex shrink-0 items-center gap-2">{actions}</div>}
      </PageBar>
      {description && <p className="text-muted-foreground text-sm">{description}</p>}
      {toolbar}
    </header>
  );
}

function TitleIcon({ icon }: { icon: ReactNode }) {
  return (
    <span className="text-muted-foreground flex shrink-0 items-center [&_svg]:size-4">{icon}</span>
  );
}

/** The bar across the top of a page. In a narrow page it is a plain row above the content. */
export function PageBar({ children }: { children: ReactNode }) {
  const { bar: slot } = usePageSlots();
  const bar = (
    <div
      data-slot="page-bar"
      data-mobile-bar
      className={cn(
        "bg-background sticky top-0 z-10 -mx-4 -mt-4 flex h-11 shrink-0 items-center gap-2 border-b px-4 text-sm font-medium group-data-[width=narrow]/page:static group-data-[width=narrow]/page:m-0 group-data-[width=narrow]/page:h-auto group-data-[width=narrow]/page:min-h-8 group-data-[width=narrow]/page:border-0 group-data-[width=narrow]/page:p-0",
        slot && "static m-0 h-12 border-0",
      )}
    >
      <SidebarButton />
      {children}
    </div>
  );
  return slot ? createPortal(bar, slot) : bar;
}
