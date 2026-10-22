import { CheckIcon, XIcon } from "lucide-react";
import type { Issue, StateRef, TeamDetail } from "@/api/ops.gen";
import { useTeamDetail } from "@/components/issues/picker-sources";
import { useIssuePatch } from "@/components/issues/use-issue-actions";
import { Button } from "@/components/ui/button";
import { SimpleTooltip } from "@/components/ui/tooltip";
import { useWorkspace } from "@/components/workspace/workspace-provider";

/** Where accepting moves an issue: the team's default status, else its first backlog or todo. */
export function acceptStateOf(team: TeamDetail | undefined): StateRef | undefined {
  if (!team) return undefined;
  if (team.defaultIssueState && team.defaultIssueState.type !== "triage") {
    return team.defaultIssueState;
  }
  return (
    team.states.find((state) => state.type === "backlog") ??
    team.states.find((state) => state.type === "unstarted")
  );
}

/** Accepting or declining an issue in triage. */
export function useTriage(issue: Pick<Issue, "id" | "team" | "priority" | "labels">) {
  const team = useTeamDetail(issue.team.id);
  const patch = useIssuePatch();
  const target = acceptStateOf(team);
  const declined = team?.states.find((state) => state.type === "canceled");
  const needsPriority = Boolean(team?.requirePriorityToLeaveTriage && issue.priority === 0);
  return {
    target,
    needsPriority,
    accept: target && !needsPriority ? () => patch(issue, { state: target }) : undefined,
    decline: declined ? () => patch(issue, { state: declined }) : undefined,
  };
}

/** Compact accept and decline buttons for a triage row. */
export function TriageActions({ issue }: { issue: Issue }) {
  const { canEdit } = useWorkspace();
  const { target, needsPriority, accept, decline } = useTriage(issue);
  if (!canEdit) return null;
  const acceptLabel = needsPriority
    ? "Set a priority to accept"
    : target
      ? `Accept into ${target.name}`
      : "Accept";
  return (
    <>
      <SimpleTooltip content={acceptLabel}>
        <Button
          variant="ghost"
          size="icon"
          forceMinSize={false}
          aria-label={acceptLabel}
          disabled={!accept}
          className="size-6"
          onClick={accept}
        >
          <CheckIcon className="size-4" />
        </Button>
      </SimpleTooltip>
      <SimpleTooltip content="Decline">
        <Button
          variant="ghost"
          size="icon"
          forceMinSize={false}
          aria-label="Decline"
          disabled={!decline}
          className="size-6"
          onClick={decline}
        >
          <XIcon className="size-4" />
        </Button>
      </SimpleTooltip>
    </>
  );
}
