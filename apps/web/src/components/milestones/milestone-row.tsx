import { Link } from "@tanstack/react-router";

import type { GitHubMilestone, Milestone } from "@/api/ops.gen";
import { GithubIcon } from "@/components/icons/github";
import {
  isMilestoneLate,
  MilestoneIcon,
  milestoneIssueCount,
} from "@/components/milestones/milestone-status";
import { ProgressBar } from "@/components/projects/progress-bar";
import { formatProjectDate } from "@/components/projects/project-dates";
import { badgeVariants } from "@/components/ui/badge";
import { cn } from "@/components/ui/utils";
import { useWorkspace } from "@/components/workspace/workspace-provider";

export function MilestoneListHeader() {
  return (
    <div
      aria-hidden
      className="text-muted-foreground dank:type-label-medium dank:border-0 dank:group-data-flush/list:px-4 flex h-8 w-full items-center gap-3 border-b px-3 text-xs group-data-flush/list:px-6"
    >
      <span className="min-w-0 flex-1">Name</span>
      <span className="hidden w-16 shrink-0 text-right sm:block">Issues</span>
      <span className="hidden w-24 shrink-0 sm:block">Target</span>
      <span className="w-32 shrink-0">Progress</span>
    </div>
  );
}

export function MilestoneRow({ milestone }: { milestone: Milestone }) {
  const { workspace } = useWorkspace();
  return (
    <div
      role="listitem"
      data-interactive
      className="has-hover:hover:bg-muted has-[a:focus-visible]:bg-muted relative flex h-9 w-full items-center gap-3 border-b px-3 text-sm group-data-flush/list:h-11 group-data-flush/list:px-6 last:border-b-0"
    >
      <span className="flex min-w-0 flex-1 items-center gap-2">
        <MilestoneIcon milestone={milestone} />
        <Link
          to="/$workspace/milestone/$id"
          params={{ workspace: workspace.urlKey, id: milestone.id }}
          className="dank:font-normal truncate font-medium outline-hidden after:absolute after:inset-0"
        >
          {milestone.name}
        </Link>
        <GithubMilestoneLinks github={milestone.github} className="relative hidden md:flex" />
      </span>
      <span className="text-muted-foreground hidden w-16 shrink-0 text-right text-xs tabular-nums sm:block">
        {milestoneIssueCount(milestone)}
      </span>
      <span
        data-late={isMilestoneLate(milestone) || undefined}
        className="text-muted-foreground data-late:text-destructive hidden w-24 shrink-0 truncate text-xs tabular-nums sm:block"
      >
        {milestone.targetDate && formatProjectDate(milestone.targetDate)}
      </span>
      <ProgressBar
        className="w-32 shrink-0"
        value={milestone.progress}
        label={`${milestone.name} progress`}
      />
    </div>
  );
}

/** The GitHub milestones it syncs with, each a chip linking to it. */
export function GithubMilestoneLinks({
  github,
  className,
}: {
  github: GitHubMilestone[];
  className?: string;
}) {
  if (github.length === 0) return null;
  return (
    <span className={cn("flex min-w-0 items-center gap-1.5", className)}>
      {github.map((item) => (
        <a
          key={item.url}
          href={item.url}
          target="_blank"
          rel="noreferrer noopener"
          data-slot="badge"
          aria-label={`${item.repository} milestone ${item.number} on GitHub`}
          className={cn(
            badgeVariants({ variant: "muted" }),
            "has-hover:hover:text-foreground focus-visible:text-foreground font-normal outline-hidden",
          )}
        >
          <GithubIcon />
          <span className="truncate">{repositoryName(item.repository)}</span>
        </a>
      ))}
    </span>
  );
}

function repositoryName(fullName: string) {
  return fullName.split("/").at(-1) ?? fullName;
}
