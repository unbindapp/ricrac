import { useState } from "react";

import type { IssueDetail, IssueRef } from "@/api/ops.gen";
import { ArrowRightLeftIcon, XIcon } from "@/components/icons/symbols.gen";
import { IssueSearchPicker } from "@/components/issues/pickers";
import { useOpenIssue } from "@/components/issues/sub-issues";
import { List } from "@/components/layout/list";
import { StateIcon } from "@/components/shared/state-icon";
import { Button } from "@/components/ui/button";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { type TIssueChanges, useUpdateIssueMutation } from "@/lib/queries/issues";

type TRelationKind = "blocks" | "blockedBy" | "related" | "duplicateOf";

const KINDS: { kind: TRelationKind; label: string }[] = [
  { kind: "blockedBy", label: "Blocked by" },
  { kind: "blocks", label: "Blocks" },
  { kind: "related", label: "Related" },
  { kind: "duplicateOf", label: "Duplicate of" },
];

function addChange(kind: TRelationKind, other: string): TIssueChanges {
  switch (kind) {
    case "blocks":
      return { blocks: [other] };
    case "blockedBy":
      return { blockedBy: [other] };
    case "related":
      return { relatedTo: [other] };
    case "duplicateOf":
      return { duplicateOf: other };
  }
}

type TRow = { key: string; label: string; issue: IssueRef; remove?: TIssueChanges };

function relationRows(issue: IssueDetail): TRow[] {
  const { relations } = issue;
  return [
    ...relations.blockedBy.map((other) => ({
      key: `blockedBy-${other.id}`,
      label: "Blocked by",
      issue: other,
      remove: { removeBlockedBy: [other.id] },
    })),
    ...relations.blocks.map((other) => ({
      key: `blocks-${other.id}`,
      label: "Blocks",
      issue: other,
      remove: { removeBlocks: [other.id] },
    })),
    ...relations.related.map((other) => ({
      key: `related-${other.id}`,
      label: "Related to",
      issue: other,
      remove: { removeRelatedTo: [other.id] },
    })),
    ...(relations.duplicateOf
      ? [
          {
            key: `duplicateOf-${relations.duplicateOf.id}`,
            label: "Duplicate of",
            issue: relations.duplicateOf,
            remove: { duplicateOf: null },
          },
        ]
      : []),
    ...relations.duplicates.map((other) => ({
      key: `duplicates-${other.id}`,
      label: "Duplicated by",
      issue: other,
    })),
  ];
}

export function hasRelations(issue: IssueDetail) {
  return relationRows(issue).length > 0;
}

/** Picks the kind of relation, then the issue. */
export function AddRelationPicker({
  issue,
  label = "Add relation",
}: {
  issue: IssueDetail;
  label?: string;
}) {
  const { workspace } = useWorkspace();
  const [kind, setKind] = useState<TRelationKind>("related");
  const { mutate: updateIssue } = useUpdateIssueMutation(workspace.id);
  const related = relationRows(issue).map((row) => row.issue.id);
  return (
    <IssueSearchPicker
      label="Add relation"
      exclude={[issue.id, ...related]}
      header={
        <div role="radiogroup" aria-label="Relation" className="flex flex-wrap gap-1 border-b p-2">
          {KINDS.map((item) => (
            <Button
              key={item.kind}
              role="radio"
              aria-checked={kind === item.kind}
              variant="ghost"
              size="sm"
              forceMinSize={false}
              data-active={kind === item.kind || undefined}
              onMouseDown={(event) => event.preventDefault()}
              className="data-active:bg-accent data-active:text-foreground text-muted-foreground font-normal"
              onClick={() => setKind(item.kind)}
            >
              {item.label}
            </Button>
          ))}
        </div>
      }
      onSelect={(other) => {
        if (!other) return;
        updateIssue({ id: issue.id, changes: addChange(kind, other.id) });
      }}
      trigger={
        <Button variant="ghost" size="sm" className="text-muted-foreground font-normal">
          <ArrowRightLeftIcon className="size-4" />
          {label}
        </Button>
      }
    />
  );
}

export function IssueRelations({ issue }: { issue: IssueDetail }) {
  const { workspace, canEdit } = useWorkspace();
  const open = useOpenIssue();
  const { mutate: updateIssue } = useUpdateIssueMutation(workspace.id);
  const rows = relationRows(issue);
  if (rows.length === 0) return null;
  return (
    <section aria-label="Relations" className="flex w-full flex-col gap-2">
      <div className="flex min-h-7 items-center gap-2">
        <h2 className="text-sm font-semibold">Relations</h2>
        <span className="flex-1" />
        {canEdit && <AddRelationPicker issue={issue} label="Add" />}
      </div>
      <List>
        {rows.map((row) => (
          <div
            key={row.key}
            role="listitem"
            tabIndex={0}
            onClick={() => open(row.issue.identifier)}
            onKeyDown={(event) =>
              event.key === "Enter" &&
              event.target === event.currentTarget &&
              open(row.issue.identifier)
            }
            className="group/row has-hover:hover:bg-accent focus-visible:ring-primary/8-10 flex h-9 w-full cursor-default items-center gap-2 border-b px-3 text-sm outline-hidden last:border-b-0 focus-visible:ring-1 focus-visible:ring-inset"
          >
            <span className="text-muted-foreground w-24 shrink-0 truncate text-xs">
              {row.label}
            </span>
            <StateIcon
              type={row.issue.state.type}
              color={row.issue.state.color}
              name={row.issue.state.name}
            />
            <span className="text-muted-foreground w-16 shrink-0 truncate font-mono text-xs">
              {row.issue.identifier}
            </span>
            <span className="min-w-0 flex-1 truncate">{row.issue.title}</span>
            {canEdit && row.remove && (
              <Button
                variant="ghost"
                size="icon"
                forceMinSize={false}
                aria-label={`Remove relation to ${row.issue.identifier}`}
                className="size-6 opacity-0 group-hover/row:opacity-100 focus-visible:opacity-100"
                onClick={(event) => {
                  event.stopPropagation();
                  if (row.remove) updateIssue({ id: issue.id, changes: row.remove });
                }}
              >
                <XIcon className="size-3.5" />
              </Button>
            )}
          </div>
        ))}
      </List>
    </section>
  );
}
