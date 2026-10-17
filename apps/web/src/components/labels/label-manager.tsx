import { useQuery } from "@tanstack/react-query";
import { ChevronRightIcon, FolderPlusIcon, PlusIcon, SearchIcon, TagIcon } from "lucide-react";
import { type ReactNode, useState } from "react";

import { LabelDialog, type TLabelDialogTarget } from "@/components/labels/label-dialog";
import { LabelRowMenu } from "@/components/labels/label-row-menu";
import { EmptyState } from "@/components/layout/empty-state";
import { List } from "@/components/layout/list";
import { ListStates } from "@/components/layout/list-states";
import { SettingsSection } from "@/components/layout/settings-section";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { cn } from "@/components/ui/utils";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { labelsQuery, labelTree, type TLabel, type TLabelNode } from "@/lib/queries/labels";

const SEARCH_THRESHOLD = 12;

type TProps = {
  /** The team whose labels to manage; null for the workspace labels. */
  team: { id: string; name: string } | null;
  canEdit: boolean;
  title: string;
  description?: ReactNode;
};

const inScope = (label: TLabel, teamId: string | null) => (label.team?.id ?? null) === teamId;

/** The labels of the scope as groups with their labels; a team's labels also show in workspace groups. */
function scopedTree(labels: TLabel[], teamId: string | null, showRetired: boolean, query: string) {
  const visible = (label: TLabel) =>
    (showRetired || !label.retiredAt) && label.name.toLowerCase().includes(query);
  return labelTree(labels).flatMap((node): TLabelNode[] => {
    const own = inScope(node, teamId);
    if (!node.isGroup) return own && visible(node) ? [node] : [];
    const children = node.labels.filter((child) => inScope(child, teamId) && visible(child));
    const groupMatches = own && (showRetired || !node.retiredAt);
    if (!groupMatches && children.length === 0) return [];
    if (query && !node.name.toLowerCase().includes(query) && children.length === 0) return [];
    return [{ ...node, labels: children }];
  });
}

export function LabelManager({ team, canEdit, title, description }: TProps) {
  const { workspace } = useWorkspace();
  const teamId = team?.id ?? null;
  const { data, isPending, error, refetch } = useQuery(
    labelsQuery(workspace.id, { team: teamId, includeRetired: true }),
  );
  const [showRetired, setShowRetired] = useState(false);
  const [query, setQuery] = useState("");
  const [dialog, setDialog] = useState<TLabelDialogTarget | null>(null);

  const labels = data ?? [];
  const own = labels.filter((label) => inScope(label, teamId));
  const retiredCount = own.filter((label) => label.retiredAt).length;
  const tree = scopedTree(labels, teamId, showRetired, query.trim().toLowerCase());
  const groups = labels.filter(
    (label) => label.isGroup && !label.retiredAt && (inScope(label, teamId) || !label.team),
  );
  const open = (target: TLabelDialogTarget) => setDialog(target);

  const createButtons = canEdit && (
    <>
      <Button variant="outline" size="sm" onClick={() => open({ mode: "create", isGroup: true })}>
        <FolderPlusIcon className="size-4" />
        New group
      </Button>
      <Button size="sm" onClick={() => open({ mode: "create", isGroup: false })}>
        <PlusIcon className="size-4" />
        New label
      </Button>
    </>
  );
  const actions = own.length > 0 && (
    <>
      {retiredCount > 0 && (
        <Button
          variant="ghost"
          size="sm"
          aria-pressed={showRetired}
          onClick={() => setShowRetired((shown) => !shown)}
          className="text-muted-foreground"
        >
          {showRetired ? "Hide retired" : `${retiredCount} retired`}
        </Button>
      )}
      {createButtons}
    </>
  );

  return (
    <SettingsSection title={title} description={description} actions={actions}>
      <ListStates
        isPending={isPending}
        error={error}
        onRetry={() => void refetch()}
        isEmpty={own.length === 0}
        empty={{
          icon: TagIcon,
          title: "No labels yet",
          description: canEdit
            ? "Labels sort issues by type, area or anything else. Groups gather related labels, like Type: Bug, Feature."
            : "Members of the workspace add labels here.",
          action: createButtons,
        }}
      >
        {own.length > SEARCH_THRESHOLD && (
          <div className="relative max-w-xs">
            <SearchIcon className="text-muted-foreground pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2" />
            <Input
              value={query}
              onChange={(event) => setQuery(event.target.value)}
              placeholder="Filter labels"
              aria-label="Filter labels"
              className="pl-9"
            />
          </div>
        )}
        {tree.length === 0 ? (
          <EmptyState icon={SearchIcon} title="No matching labels" />
        ) : (
          <List>
            {tree.map((node) =>
              node.isGroup ? (
                <GroupRows
                  key={node.id}
                  group={node}
                  teamId={teamId}
                  canEdit={canEdit}
                  onOpen={open}
                />
              ) : (
                <LabelRow key={node.id} label={node} canEdit={canEdit} onOpen={open} />
              ),
            )}
          </List>
        )}
      </ListStates>
      <LabelDialog target={dialog} onClose={() => setDialog(null)} team={team} groups={groups} />
    </SettingsSection>
  );
}

type TRowProps = {
  canEdit: boolean;
  onOpen: (target: TLabelDialogTarget) => void;
};

function GroupRows({
  group,
  teamId,
  ...props
}: { group: TLabelNode; teamId: string | null } & TRowProps) {
  const [isOpen, setIsOpen] = useState(true);
  const isOwn = inScope(group, teamId);
  const count = group.labels.length;
  return (
    <>
      <Row
        label={group}
        leading={
          <Button
            variant="ghost"
            size="icon"
            forceMinSize={false}
            aria-expanded={isOpen}
            aria-label={isOpen ? `Collapse ${group.name}` : `Expand ${group.name}`}
            onClick={() => setIsOpen((open) => !open)}
            className="text-muted-foreground -ml-1 size-6"
          >
            <ChevronRightIcon
              data-open={isOpen || undefined}
              className="size-4 transition-transform data-open:rotate-90"
            />
          </Button>
        }
        meta={
          <>
            <Badge variant="muted">
              {group.groupType === "multiSelect" ? "Multi select" : "Single select"}
            </Badge>
            {!isOwn && <span className="text-muted-foreground text-xs">Workspace group</span>}
          </>
        }
        trailing={`${count} ${count === 1 ? "label" : "labels"}`}
        menu={props.canEdit && isOwn && <LabelRowMenu label={group} onOpen={props.onOpen} />}
      />
      {isOpen &&
        group.labels.map((child) => <LabelRow key={child.id} label={child} nested {...props} />)}
    </>
  );
}

function LabelRow({
  label,
  nested,
  canEdit,
  onOpen,
}: { label: TLabel; nested?: boolean } & TRowProps) {
  return (
    <Row
      label={label}
      nested={nested}
      trailing={
        label.issueCount > 0 && `${label.issueCount} ${label.issueCount === 1 ? "issue" : "issues"}`
      }
      menu={canEdit && <LabelRowMenu label={label} onOpen={onOpen} />}
    />
  );
}

function Row({
  label,
  nested,
  leading,
  meta,
  trailing,
  menu,
}: {
  label: TLabel;
  nested?: boolean;
  leading?: ReactNode;
  meta?: ReactNode;
  trailing?: ReactNode;
  menu?: ReactNode;
}) {
  return (
    <div
      role="listitem"
      data-retired={label.retiredAt ? true : undefined}
      data-nested={nested || undefined}
      className="data-retired:text-muted-foreground flex h-9 w-full items-center gap-2 border-b px-3 text-sm last:border-b-0 data-nested:pl-10"
    >
      {leading ?? <span aria-hidden className="-ml-1 size-6 shrink-0" />}
      <span
        aria-hidden
        className={cn("size-2.5 shrink-0 rounded-full", label.retiredAt && "opacity-50")}
        style={{ backgroundColor: label.color }}
      />
      <span className={cn("shrink-0 truncate", !label.retiredAt && "font-medium")}>
        {label.name}
      </span>
      {meta}
      {label.retiredAt && <Badge variant="muted">Retired</Badge>}
      {label.description && (
        <span className="text-muted-foreground hidden min-w-0 truncate sm:inline">
          {label.description}
        </span>
      )}
      <span className="text-muted-foreground ml-auto shrink-0 text-xs">{trailing}</span>
      {menu}
    </div>
  );
}
