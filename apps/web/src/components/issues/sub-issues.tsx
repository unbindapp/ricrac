import { ListTreeIcon, PlusIcon } from "lucide-react";

import type { IssueDetail } from "@/api/ops.gen";
import { useIsPeekable, useIssuePeek } from "@/components/issues/issue-peek";
import { useNewIssueStore } from "@/components/issues/new-issue-store";
import {
  AssigneePicker,
  IssueSearchPicker,
  StatePicker,
  toIssueRef,
} from "@/components/issues/pickers";
import { useIssuePatch, useOpenIssuePage } from "@/components/issues/use-issue-actions";
import { List } from "@/components/layout/list";
import { Button } from "@/components/ui/button";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { useUpdateIssueMutation } from "@/lib/queries/issues";

const CLOSED = ["completed", "canceled"];
const NEW_SUB_ISSUE = "new";

export function useOpenIssue() {
  const peek = useIssuePeek();
  const isPeekable = useIsPeekable();
  const openPage = useOpenIssuePage();
  return (identifier: string) => {
    if (peek && isPeekable) {
      peek.openPeek(identifier);
      return;
    }
    void openPage(identifier);
  };
}

/** Make a new sub-issue, or make an existing issue one. */
export function AddSubIssueMenu({
  issue,
  label = "Add sub-issue",
}: {
  issue: IssueDetail;
  label?: string;
}) {
  const { workspace } = useWorkspace();
  const openNewIssue = useNewIssueStore((state) => state.open);
  const { mutate: updateIssue } = useUpdateIssueMutation(workspace.id);
  return (
    <IssueSearchPicker
      label="Add sub-issue"
      exclude={[issue.id, ...issue.children.map((child) => child.id)]}
      extraOptions={[
        {
          value: NEW_SUB_ISSUE,
          label: "Create a new sub-issue…",
          icon: <PlusIcon className="size-4" />,
        },
      ]}
      onOption={() =>
        openNewIssue({
          teamId: issue.team.id,
          draft: {
            parent: toIssueRef(issue),
            project: issue.project,
            projectMilestone: issue.projectMilestone,
            cycle: issue.cycle,
          },
        })
      }
      onSelect={(child) => {
        if (!child) return;
        updateIssue({
          id: child.id,
          changes: { parentId: issue.id },
          optimistic: { parent: toIssueRef(issue) },
        });
      }}
      trigger={
        <Button variant="ghost" size="sm" className="text-muted-foreground font-normal">
          <ListTreeIcon className="size-4" />
          {label}
        </Button>
      }
    />
  );
}

export function SubIssues({ issue }: { issue: IssueDetail }) {
  const { canEdit } = useWorkspace();
  const patch = useIssuePatch();
  const open = useOpenIssue();
  const children = issue.children;
  if (children.length === 0) return null;
  const done = children.filter((child) => CLOSED.includes(child.state.type)).length;

  return (
    <section aria-label="Sub-issues" className="flex w-full flex-col gap-2">
      <div className="flex min-h-7 items-center gap-2">
        <h2 className="text-sm font-semibold">Sub-issues</h2>
        <span className="text-muted-foreground text-xs">
          {done}/{children.length} done
        </span>
        <span className="flex-1" />
        {canEdit && <AddSubIssueMenu issue={issue} label="Add" />}
      </div>
      <List>
        {children.map((child) => (
          <div
            key={child.id}
            role="listitem"
            tabIndex={0}
            onClick={() => open(child.identifier)}
            onKeyDown={(event) =>
              event.key === "Enter" &&
              event.target === event.currentTarget &&
              open(child.identifier)
            }
            className="group/row has-hover:hover:bg-accent focus-visible:ring-primary/8-10 flex h-9 w-full cursor-default items-center gap-2 border-b px-3 text-sm outline-hidden last:border-b-0 focus-visible:ring-1 focus-visible:ring-inset"
          >
            <StatePicker
              appearance="icon"
              disabled={!canEdit}
              teamId={issue.team.id}
              state={child.state}
              onChange={(state) => patch(child, { state })}
            />
            <span className="text-muted-foreground w-16 shrink-0 truncate font-mono text-xs">
              {child.identifier}
            </span>
            <span className="min-w-0 flex-1 truncate">{child.title}</span>
            <AssigneePicker
              appearance="icon"
              disabled={!canEdit}
              assignee={child.assignee}
              className={
                child.assignee
                  ? undefined
                  : "opacity-0 group-hover/row:opacity-100 focus-visible:opacity-100 data-popup-open:opacity-100"
              }
              onChange={(assignee) => patch(child, { assignee })}
            />
          </div>
        ))}
      </List>
    </section>
  );
}
