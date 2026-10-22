import { Link } from "@tanstack/react-router";

import type { Initiative } from "@/api/ops.gen";
import { BoxIcon } from "@/components/icons";
import { InitiativeIcon } from "@/components/initiatives/initiative-status";
import { HealthIcon, healthLabels } from "@/components/projects/health-badge";
import { PersonAvatar } from "@/components/projects/people";
import { ProgressBar } from "@/components/projects/progress-bar";
import { formatProjectDate } from "@/components/projects/project-dates";
import { SimpleTooltip } from "@/components/ui/tooltip";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { entitySlug } from "@/lib/helpers/entity-slug";

export function InitiativeListHeader() {
  return (
    <div
      aria-hidden
      className="text-muted-foreground flex h-8 w-full items-center gap-3 border-b px-3 text-xs group-data-flush/list:px-6"
    >
      <span className="min-w-0 flex-1">Name</span>
      <span className="hidden w-24 shrink-0 md:block">Health</span>
      <span className="hidden w-12 shrink-0 sm:block">Projects</span>
      <span className="w-12 shrink-0 text-center">Owner</span>
      <span className="hidden w-24 shrink-0 sm:block">Target</span>
      <span className="w-32 shrink-0">Progress</span>
    </div>
  );
}

type TProps = {
  initiative: Initiative;
  depth?: number;
  /** Name the parent, when the row isn't shown under it. */
  showParent?: boolean;
};

export function InitiativeRow({ initiative, depth = 0, showParent }: TProps) {
  const { workspace } = useWorkspace();
  const owner = initiative.owner;
  return (
    <Link
      role="listitem"
      to="/$workspace/initiative/$slug"
      params={{ workspace: workspace.urlKey, slug: entitySlug(initiative) }}
      className="has-hover:hover:bg-muted focus-visible:bg-muted flex h-9 w-full items-center gap-3 border-b px-3 text-sm outline-hidden group-data-flush/list:h-11 group-data-flush/list:px-6 last:border-b-0"
    >
      <span
        data-depth={Math.min(depth, 3)}
        className="flex min-w-0 flex-1 items-center gap-2 data-[depth=1]:pl-6 data-[depth=2]:pl-12 data-[depth=3]:pl-16"
      >
        <InitiativeIcon initiative={initiative} />
        <span className="truncate font-medium">{initiative.name}</span>
        {showParent && initiative.parent && (
          <span className="text-muted-foreground hidden min-w-0 truncate text-xs md:inline">
            in {initiative.parent.name}
          </span>
        )}
        {!showParent && initiative.summary && (
          <span className="text-muted-foreground hidden min-w-0 truncate text-xs lg:inline">
            {initiative.summary}
          </span>
        )}
      </span>
      <span className="hidden w-24 shrink-0 items-center gap-1.5 text-xs md:flex">
        {initiative.health && (
          <>
            <HealthIcon health={initiative.health} />
            <span className="text-muted-foreground truncate">
              {healthLabels[initiative.health]}
            </span>
          </>
        )}
      </span>
      <span className="text-muted-foreground hidden w-12 shrink-0 items-center gap-1.5 text-xs tabular-nums sm:flex">
        {initiative.projectCount > 0 && (
          <>
            <BoxIcon className="size-3.5" aria-hidden />
            {initiative.projectCount}
          </>
        )}
      </span>
      <span className="flex w-12 shrink-0 justify-center">
        {owner && (
          <SimpleTooltip content={`Owner: ${owner.name}`}>
            <span className="flex">
              <PersonAvatar user={owner} size="xs" />
            </span>
          </SimpleTooltip>
        )}
      </span>
      <span className="text-muted-foreground hidden w-24 shrink-0 truncate text-xs tabular-nums sm:block">
        {initiative.targetDate &&
          formatProjectDate(initiative.targetDate, initiative.targetDateResolution)}
      </span>
      <span className="w-32 shrink-0">
        {initiative.projectCount > 0 && (
          <ProgressBar value={initiative.progress} label={`${initiative.name} progress`} />
        )}
      </span>
    </Link>
  );
}
