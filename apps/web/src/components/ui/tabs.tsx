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
        "dank:bg-surface-container-highest dank:mx-0 dank:w-fit dank:rounded-full dank:p-0.5 -mx-1 flex items-center",
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
        "hit-gap-x bg-muted text-muted-foreground has-hover:hover:text-foreground data-active:bg-accent data-active:text-foreground focus-visible:ring-primary/8-10 dank:has-hover:hover:bg-accent dank:data-active:bg-secondary dank:data-active:text-secondary-foreground dank:focus-visible:ring-primary dank:h-8 dank:rounded-full dank:bg-transparent dank:pr-3 dank:pl-3 dank:transition-[background-color,color] dank:focus-visible:ring-2 flex h-7 items-center gap-1.5 rounded-md px-2.5 text-sm font-medium whitespace-nowrap outline-hidden select-none focus-visible:ring-1 [&_svg]:size-4 [&_svg]:shrink-0",
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
