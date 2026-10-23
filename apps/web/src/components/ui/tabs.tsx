import { Tabs as TabsPrimitive } from "@base-ui/react/tabs";
import { createContext, useContext } from "react";

import { cn } from "@/components/ui/utils";

type TTabsVariant = "tabs" | "segmented";

const TabsVariantContext = createContext<TTabsVariant>("tabs");

function Tabs({ className, ...props }: TabsPrimitive.Root.Props) {
  return (
    <TabsPrimitive.Root
      data-slot="tabs"
      className={cn("flex flex-col gap-4", className)}
      {...props}
    />
  );
}

function TabsList({
  className,
  children,
  variant = "tabs",
  ...props
}: TabsPrimitive.List.Props & { variant?: TTabsVariant }) {
  return (
    <TabsVariantContext value={variant}>
      <TabsPrimitive.List
        data-slot="tabs-list"
        data-variant={variant}
        className={cn(
          "-mx-1 flex items-center",
          variant === "tabs" &&
            "dank:relative dank:mx-0 dank:h-12 dank:items-stretch dank:shadow-[inset_0_-1px_0_var(--m3-outline-variant)]",
          className,
        )}
        {...props}
      >
        {children}
        {variant === "tabs" && (
          <TabsPrimitive.Indicator
            data-slot="tabs-indicator"
            className="bg-m3-primary dank:block ease-spring-default-spatial absolute bottom-0 left-[calc(var(--active-tab-left)+1rem)] hidden h-[3px] w-[calc(var(--active-tab-width)-2rem)] rounded-[3px] transition-[left,width] duration-(--spring-default-spatial-duration)"
          />
        )}
      </TabsPrimitive.List>
    </TabsVariantContext>
  );
}

const dankTab =
  "dank:relative dank:h-12 dank:min-w-22.5 dank:justify-center dank:rounded-none dank:border-0 dank:bg-transparent dank:px-4 dank:type-title-small dank:text-m3-on-surface-variant dank:state-layer dank:has-hover:hover:text-m3-on-surface dank:data-active:bg-transparent dank:data-active:text-m3-primary dank:has-hover:hover:data-active:text-m3-primary";

const dankSegment = "dank:justify-center dank:px-4 dank:type-label-large dank:[&_svg]:size-5";

function TabsTab({ className, ...props }: TabsPrimitive.Tab.Props) {
  const variant = useContext(TabsVariantContext);
  return (
    <TabsPrimitive.Tab
      data-slot="tabs-tab"
      className={cn(
        "hit-gap-x bg-muted text-muted-foreground has-hover:hover:text-foreground data-active:bg-accent data-active:text-foreground focus-visible:ring-primary/8-10 flex h-7 items-center gap-1.5 rounded-md px-2.5 text-sm font-medium whitespace-nowrap outline-hidden select-none focus-visible:ring-1 [&_svg]:size-4 [&_svg]:shrink-0",
        variant === "tabs" ? dankTab : dankSegment,
        className,
      )}
      {...props}
    />
  );
}

function TabsPanel({ className, ...props }: TabsPrimitive.Panel.Props) {
  return (
    <TabsPrimitive.Panel
      data-slot="tabs-panel"
      className={cn("outline-hidden", className)}
      {...props}
    />
  );
}

export { Tabs, TabsList, TabsTab, TabsPanel };
