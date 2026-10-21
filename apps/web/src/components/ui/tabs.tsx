import { Tabs as TabsPrimitive } from "@base-ui/react/tabs";

import { cn } from "@/components/ui/utils";

function Tabs({ className, ...props }: TabsPrimitive.Root.Props) {
  return (
    <TabsPrimitive.Root
      data-slot="tabs"
      className={cn("flex flex-col gap-4", className)}
      {...props}
    />
  );
}

function TabsList({ className, children, ...props }: TabsPrimitive.List.Props) {
  return (
    <TabsPrimitive.List
      data-slot="tabs-list"
      className={cn(
        "bg-surface-container-highest flex w-fit items-center rounded-full p-0.5",
        className,
      )}
      {...props}
    >
      {children}
    </TabsPrimitive.List>
  );
}

function TabsTab({ className, ...props }: TabsPrimitive.Tab.Props) {
  return (
    <TabsPrimitive.Tab
      data-slot="tabs-tab"
      className={cn(
        "hit-gap-x text-muted-foreground has-hover:hover:bg-accent has-hover:hover:text-foreground data-active:bg-secondary data-active:text-secondary-foreground focus-visible:ring-primary flex h-8 items-center gap-1.5 rounded-full px-3 text-sm font-medium whitespace-nowrap outline-hidden transition-[background-color,color] select-none focus-visible:ring-2 [&_svg]:size-4 [&_svg]:shrink-0",
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
