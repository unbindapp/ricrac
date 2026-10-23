import { ChevronRightIcon, PlusIcon } from "@/components/icons";
import { GroupIcon } from "@/components/issues/group-icon";
import type { TIssueGroup } from "@/components/issues/grouping";
import { Button } from "@/components/ui/button";

type TProps = {
  group: TIssueGroup;
  isCollapsed: boolean;
  onToggle: () => void;
  onCreate?: () => void;
};

export function IssueGroupHeader({ group, isCollapsed, onToggle, onCreate }: TProps) {
  const title = (
    <>
      <GroupIcon meta={group.meta} />
      <span className="truncate">{group.label}</span>
      <span className="text-muted-foreground dank:type-label-large text-sm font-normal">
        {group.issues.length}
      </span>
    </>
  );
  return (
    <div
      role="listitem"
      aria-label={`${group.label}, ${group.issues.length} issues`}
      className="bg-muted dank:h-12 dank:bg-transparent dank:px-0 flex h-9 w-full items-center gap-2 px-2 text-sm sm:px-4"
    >
      <Button
        variant="ghost"
        size="sm"
        forceMinSize={false}
        aria-label={group.label}
        aria-expanded={!isCollapsed}
        onClick={onToggle}
        className="dank:size-10 dank:justify-center dank:p-0 min-w-0 justify-start px-2"
      >
        <span className="contents">
          <ChevronRightIcon
            data-open={!isCollapsed || undefined}
            className="text-muted-foreground dank:size-6 size-3.5 transition-transform data-open:rotate-90"
          />
        </span>
        <span className="dank:hidden contents">{title}</span>
      </Button>
      <span className="dank:flex dank:type-title-small hidden min-w-0 items-center gap-2">
        {title}
      </span>
      <span className="flex-1" />
      {onCreate && (
        <Button
          variant="ghost"
          size="icon"
          aria-label={`New issue in ${group.label}`}
          onClick={onCreate}
          className="text-muted-foreground"
        >
          <PlusIcon className="size-4" />
        </Button>
      )}
    </div>
  );
}
