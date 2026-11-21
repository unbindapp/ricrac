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
            "dux:relative dux:mx-0 dux:h-12 dux:items-stretch dux:shadow-[inset_0_-1px_0_var(--m3-outline-variant)]",
          className,
        )}
        {...props}
      >
        {children}
        {variant === "tabs" && (
          <TabsPrimitive.Indicator
            data-slot="tabs-indicator"
            className="bg-m3-primary dux:block ease-spring-default-spatial absolute bottom-0 left-[calc(var(--active-tab-left)+1rem)] hidden h-[3px] w-[calc(var(--active-tab-width)-2rem)] rounded-t-[3px] transition-[left,width] duration-(--spring-default-spatial-duration)"
          />
        )}
      </TabsPrimitive.List>
    </TabsVariantContext>
  );
}

const duxTab =
  "dux:relative dux:h-12 dux:min-w-22.5 dux:justify-center dux:rounded-none dux:border-0 dux:bg-transparent dux:px-4 dux:type-title-small dux:text-m3-on-surface-variant dux:state-layer dux:has-hover:hover:text-m3-on-surface dux:data-active:bg-transparent dux:data-active:text-m3-primary dux:has-hover:hover:data-active:text-m3-primary";

const duxSegment = "dux:justify-center dux:px-4 dux:type-label-large dux:[&_svg]:size-5";

function TabsTab({ className, ...props }: TabsPrimitive.Tab.Props) {
  const variant = useContext(TabsVariantContext);
  return (
    <TabsPrimitive.Tab
      data-slot="tabs-tab"
      className={cn(
        "hit-gap-x bg-muted text-muted-foreground has-hover:hover:text-foreground data-active:bg-accent data-active:text-foreground focus-visible:ring-primary/8-10 flex h-7 items-center gap-1.5 rounded-md px-2.5 text-sm font-medium whitespace-nowrap outline-hidden select-none focus-visible:ring-1 [&_svg]:size-4 [&_svg]:shrink-0",
        variant === "tabs" ? duxTab : duxSegment,
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
