import { type ReactNode } from "react";

import { ChevronRightIcon } from "@/components/icons";
import { Button } from "@/components/ui/button";
import { useUiTheme } from "@/themes/ui-theme-provider";

type TProps = {
  icon: ReactNode;
  label: string;
  count: number;
  isCollapsed: boolean;
  onToggle: () => void;
};

/** Opens and closes a group of a list; in Dank an icon button beside the group's title. */
export function GroupToggle({ icon, label, count, isCollapsed, onToggle }: TProps) {
  const isDank = useUiTheme().id === "dank";
  const chevron = (
    <ChevronRightIcon
      data-open={!isCollapsed || undefined}
      className="text-muted-foreground size-3.5 transition-transform data-open:rotate-90"
    />
  );
  if (!isDank) {
    return (
      <Button
        variant="ghost"
        size="sm"
        forceMinSize={false}
        aria-expanded={!isCollapsed}
        onClick={onToggle}
      >
        {chevron}
        {icon}
        {label}
        <span className="text-muted-foreground text-xs font-normal tabular-nums">{count}</span>
      </Button>
    );
  }
  return (
    <>
      <Button
        variant="ghost"
        size="icon"
        aria-label={`${label} ${count}`}
        aria-expanded={!isCollapsed}
        onClick={onToggle}
      >
        {chevron}
      </Button>
      <span className="type-title-small flex min-w-0 items-center gap-2">
        {icon}
        <span className="truncate">{label}</span>
        <span className="text-muted-foreground type-label-large tabular-nums">{count}</span>
      </span>
    </>
  );
}
