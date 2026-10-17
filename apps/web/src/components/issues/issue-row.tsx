import type { MouseEvent as ReactMouseEvent, ReactNode } from "react";

import type { Issue } from "@/api/ops.gen";
import type { TDisplaySettings } from "@/components/issues/display";
import { cycleName } from "@/components/issues/grouping";
import { DueDateText, ProjectChip, SlaChip, SnoozedChip } from "@/components/issues/issue-chips";
import { useProjectLookup, useTeamDetail } from "@/components/issues/picker-sources";
import { AssigneePicker, PriorityPicker, StatePicker } from "@/components/issues/pickers";
import { useIssuePatch } from "@/components/issues/use-issue-actions";
import { EstimateBadge } from "@/components/shared/estimate-badge";
import { LabelChip } from "@/components/shared/label-chip";
import { RelativeTime } from "@/components/shared/relative-time";

const MAX_ROW_LABELS = 3;

/** Empty controls stay out of sight until the row is hovered or the control is focused or open. */
export const hiddenUntilHover =
  "opacity-0 touch:opacity-100 group-hover/row:opacity-100 group-focus-within/row:opacity-100 focus-visible:opacity-100 data-popup-open:opacity-100";

type TProps = {
  issue: Issue;
  depth?: number;
  properties: TDisplaySettings["properties"];
  isSelected: boolean;
  isActive: boolean;
  isFocused?: boolean;
  canEdit: boolean;
  actions?: ReactNode;
  onClick: (event: ReactMouseEvent, issue: Issue) => void;
  onFocus: (issue: Issue) => void;
};

export function IssueRow({
  issue,
  depth = 0,
  properties,
  isSelected,
  isActive,
  isFocused,
  canEdit,
  actions,
  onClick,
  onFocus,
}: TProps) {
  const patch = useIssuePatch();
  const team = useTeamDetail(properties.includes("estimate") ? issue.team.id : undefined);
  const project = useProjectLookup().get(issue.project?.id ?? "");
  const show = (property: TDisplaySettings["properties"][number]) => properties.includes(property);
  const labels = issue.labels.slice(0, MAX_ROW_LABELS);
  const hiddenLabels = issue.labels.length - labels.length;

  return (
    <div
      role="listitem"
      tabIndex={0}
      data-issue-row={issue.id}
      aria-selected={isSelected}
      aria-label={`${issue.identifier} ${issue.title}`}
      data-selected={isSelected || undefined}
      data-active={isActive || undefined}
      data-focused={isFocused || undefined}
      onClick={(event) => onClick(event, issue)}
      onKeyDown={(event) => {
        if (event.key !== "Enter" || event.target !== event.currentTarget) return;
        onClick(event as unknown as ReactMouseEvent, issue);
      }}
      onMouseEnter={() => onFocus(issue)}
      onFocus={() => onFocus(issue)}
      className="group/row has-hover:hover:bg-muted data-focused:bg-muted data-active:bg-accent data-selected:bg-primary/2-10 data-selected:has-hover:hover:bg-primary/3-10 focus-visible:ring-primary/8-10 flex h-11 w-full cursor-default items-center gap-2 px-4 text-sm outline-hidden focus-visible:ring-1 focus-visible:ring-inset sm:px-6"
    >
      {show("priority") && (
        <PriorityPicker
          appearance="icon"
          disabled={!canEdit}
          priority={issue.priority}
          className={issue.priority === 0 ? hiddenUntilHover : undefined}
          onChange={(priority) => patch(issue, { priority })}
        />
      )}
      {show("identifier") && (
        <span className="text-muted-foreground w-16 shrink-0 truncate font-mono text-xs max-sm:hidden">
          {issue.identifier}
        </span>
      )}
      {show("state") && (
        <StatePicker
          appearance="icon"
          disabled={!canEdit}
          teamId={issue.team.id}
          state={issue.state}
          onChange={(state) => patch(issue, { state })}
        />
      )}
      <span
        data-nested={depth > 0 || undefined}
        className="min-w-0 flex-1 truncate font-medium data-nested:pl-4"
      >
        {issue.title}
      </span>
      <span className="hidden min-w-0 shrink items-center justify-end gap-1.5 overflow-hidden sm:flex">
        <SnoozedChip issue={issue} />
        <SlaChip issue={issue} />
        {show("labels") &&
          labels.map((label) => <LabelChip key={label.id} name={label.name} color={label.color} />)}
        {show("labels") && hiddenLabels > 0 && (
          <span className="text-muted-foreground text-xs">+{hiddenLabels}</span>
        )}
        {show("project") && issue.project && (
          <ProjectChip
            project={{ name: issue.project.name, icon: project?.icon, color: project?.color }}
          />
        )}
        {show("cycle") && issue.cycle && (
          <span className="text-muted-foreground shrink-0 text-xs whitespace-nowrap">
            {cycleName(issue.cycle)}
          </span>
        )}
        {show("estimate") && team && (
          <EstimateBadge value={issue.estimate} type={team.estimateType} />
        )}
        {show("due_date") && <DueDateText issue={issue} />}
        {show("updated_at") && (
          <span className="text-muted-foreground shrink-0 text-xs">
            <RelativeTime value={issue.updatedAt} />
          </span>
        )}
      </span>
      {actions && (
        <span
          className={`flex shrink-0 items-center gap-1 ${hiddenUntilHover}`}
          onClick={(event) => event.stopPropagation()}
        >
          {actions}
        </span>
      )}
      {show("assignee") && (
        <AssigneePicker
          appearance="icon"
          disabled={!canEdit}
          assignee={issue.assignee}
          className={issue.assignee ? undefined : hiddenUntilHover}
          onChange={(assignee) => patch(issue, { assignee })}
        />
      )}
    </div>
  );
}
