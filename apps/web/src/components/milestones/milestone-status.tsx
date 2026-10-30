import type { Milestone } from "@/api/ops.gen";
import { DiamondIcon } from "@/components/icons";
import { isPastTarget } from "@/components/projects/project-dates";
import { cn } from "@/components/ui/utils";

/** Closed, or every issue in it finished. */
export function isMilestoneDone(milestone: Milestone) {
  if (milestone.closedAt) return true;
  const finished = milestone.completedIssueCount + milestone.canceledIssueCount;
  return milestone.issueCount > 0 && finished === milestone.issueCount;
}

export function isMilestoneLate(milestone: Milestone) {
  if (isMilestoneDone(milestone) || !milestone.targetDate) return false;
  return isPastTarget(milestone.targetDate);
}

/** Completed out of the issues that count, like "3 / 5"; empty without issues. */
export function milestoneIssueCount(milestone: Milestone) {
  if (milestone.issueCount === 0) return "";
  return `${milestone.completedIssueCount} / ${milestone.issueCount - milestone.canceledIssueCount}`;
}

export function MilestoneIcon({
  milestone,
  className,
}: {
  milestone: Milestone;
  className?: string;
}) {
  return (
    <DiamondIcon
      aria-hidden
      data-done={isMilestoneDone(milestone) || undefined}
      className={cn(
        "text-muted-foreground data-done:fill-primary data-done:text-primary dank:size-5 size-3.5 shrink-0",
        className,
      )}
    />
  );
}
