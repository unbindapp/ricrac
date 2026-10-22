import { useQuery } from "@tanstack/react-query";
import { parseISO } from "date-fns";
import { type ReactElement, type ReactNode, useState } from "react";
import { useDebounceValue } from "usehooks-ts";
import {
  CalendarIcon,
  CircleDashedIcon,
  CircleUserIcon,
  CornerLeftUpIcon,
  DiamondIcon,
  TagIcon,
  type LucideIcon,
  TriangleIcon,
  UsersIcon,
  XIcon,
} from "lucide-react";

import type {
  CycleRef,
  Issue,
  IssueLabel,
  IssueRef,
  LabelRef,
  MilestoneRef,
  ProjectRef,
  StateRef,
  UserRef,
} from "@/api/ops.gen";
import { cycleName, NONE_KEY, PRIORITY_ORDER, userName } from "@/components/issues/grouping";
import {
  useAvatarUrl,
  useCycles,
  useLabels,
  useMilestones,
  useProjectLookup,
  useProjects,
  useTeamDetail,
  useTeamStates,
  useWorkspaceUsers,
} from "@/components/issues/picker-sources";
import {
  BlockPageHotkeys,
  PropertyPicker,
  type TPickerOption,
} from "@/components/issues/property-picker";
import { Avatar } from "@/components/shared/avatar";
import { CycleIcon } from "@/components/shared/cycle-icon";
import { LabelChip } from "@/components/shared/label-chip";
import { PriorityIcon, priorityLabel } from "@/components/shared/priority-icon";
import { ProjectIcon } from "@/components/shared/project-icon";
import { StateIcon } from "@/components/shared/state-icon";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { cn } from "@/components/ui/utils";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { formatDate } from "@/lib/helpers/dates";
import { formatEstimate } from "@/lib/helpers/estimates";
import { searchIssuesQuery } from "@/lib/queries/issues";

/**
 * row: a line of the properties panel, empty values in muted words ("No priority").
 * compact: a chip in the new-issue dialog and the bulk bar, empty ones named by property.
 * icon: just the icon, in list rows and cards.
 */
export type TPickerAppearance = "row" | "compact" | "icon";

type TPickerControl = {
  appearance?: TPickerAppearance;
  open?: boolean;
  onOpenChange?: (open: boolean) => void;
  disabled?: boolean;
  className?: string;
};

export function PickerTrigger({
  appearance = "row",
  label,
  icon,
  text,
  muted,
  className,
  ...props
}: {
  appearance?: TPickerAppearance;
  label: string;
  icon: ReactNode;
  text: ReactNode;
  muted?: boolean;
  className?: string;
}) {
  switch (appearance) {
    case "icon":
      return (
        <Button
          variant="ghost"
          size="icon"
          forceMinSize={false}
          aria-label={label}
          className={cn("size-6 shrink-0", className)}
          {...props}
        >
          {icon}
        </Button>
      );
    case "compact":
      return (
        <Button
          variant="outline"
          size="sm"
          forceMinSize={false}
          aria-label={label}
          data-muted={muted || undefined}
          className={cn("data-muted:text-muted-foreground max-w-48 font-normal", className)}
          {...props}
        >
          {icon}
          <span className="truncate">{text}</span>
        </Button>
      );
    case "row":
      return (
        <Button
          variant="ghost"
          size="sm"
          forceMinSize={false}
          fadeOnDisabled={false}
          aria-label={label}
          data-muted={muted || undefined}
          className={cn(
            "data-muted:text-muted-foreground h-auto min-h-7 w-full min-w-0 justify-start py-1 font-normal",
            className,
          )}
          {...props}
        >
          {icon}
          <span className="min-w-0 truncate">{text}</span>
        </Button>
      );
  }
}

/** Empty values read as words in the panel ("No priority") and as the property's name on chips. */
function emptyText(appearance: TPickerAppearance, row: string, chip: string) {
  return appearance === "row" ? row : chip;
}

function MutedIcon({ icon: Icon }: { icon: LucideIcon }) {
  return <Icon className="text-muted-foreground size-4" />;
}

export function StatePicker({
  teamId,
  state,
  onChange,
  appearance = "row",
  className,
  ...control
}: TPickerControl & {
  teamId: string | undefined;
  state: StateRef | undefined;
  onChange: (state: StateRef) => void;
}) {
  const states = useTeamStates(teamId);
  const options: TPickerOption[] = states.map((item, index) => ({
    value: item.id,
    label: item.name,
    icon: <StateIcon type={item.type} color={item.color} name={item.name} />,
    shortcut: index < 9 ? String(index + 1) : undefined,
  }));
  return (
    <PropertyPicker
      label="Change status"
      options={options}
      value={state ? [state.id] : []}
      onChange={([id]) => {
        const next = states.find((item) => item.id === id);
        if (!next || next.id === state?.id) return;
        onChange({ id: next.id, name: next.name, color: next.color, type: next.type });
      }}
      {...control}
      trigger={
        <PickerTrigger
          appearance={appearance}
          className={className}
          label={state ? `Status: ${state.name}` : "Set status"}
          icon={
            state ? (
              <StateIcon type={state.type} color={state.color} name={state.name} />
            ) : (
              <MutedIcon icon={CircleDashedIcon} />
            )
          }
          text={state?.name ?? "Status"}
          muted={!state}
        />
      }
    />
  );
}

export function PriorityPicker({
  priority,
  onChange,
  appearance = "row",
  className,
  ...control
}: TPickerControl & { priority: number; onChange: (priority: number) => void }) {
  const options: TPickerOption[] = [0, ...PRIORITY_ORDER.filter((p) => p !== 0)].map((value) => ({
    value: String(value),
    label: priorityLabel(value),
    icon: <PriorityIcon priority={value} />,
    shortcut: String(value),
  }));
  return (
    <PropertyPicker
      label="Change priority"
      options={options}
      value={[String(priority)]}
      onChange={([value]) => {
        const next = Number(value);
        if (next !== priority) onChange(next);
      }}
      {...control}
      trigger={
        <PickerTrigger
          appearance={appearance}
          className={className}
          label={`Priority: ${priorityLabel(priority)}`}
          icon={<PriorityIcon priority={priority} />}
          text={
            priority === 0
              ? emptyText(appearance, "No priority", "Priority")
              : priorityLabel(priority)
          }
          muted={priority === 0}
        />
      }
    />
  );
}

type TAvatarUser = Pick<UserRef, "id" | "name" | "displayName"> & { avatarUrl?: string | null };

export function UserAvatar({
  user,
  size = "sm",
}: {
  user: TAvatarUser;
  size?: "xs" | "sm" | "md" | "lg";
}) {
  const looked = useAvatarUrl(user.avatarUrl === undefined ? user : null);
  return <Avatar id={user.id} name={userName(user)} src={user.avatarUrl ?? looked} size={size} />;
}

function userOption(user: TAvatarUser & { email?: string }): TPickerOption {
  return {
    value: user.id,
    label: userName(user),
    description:
      user.displayName && user.displayName !== user.name ? `@${user.displayName}` : undefined,
    keywords: [user.displayName, user.email ?? ""],
    icon: <UserAvatar user={user} size="xs" />,
  };
}

const toUserRef = (user: UserRef): UserRef => ({
  id: user.id,
  name: user.name,
  displayName: user.displayName,
  email: user.email,
});

export function AssigneePicker({
  assignee,
  onChange,
  appearance = "row",
  className,
  ...control
}: TPickerControl & {
  assignee: UserRef | null | undefined;
  onChange: (user: UserRef | null) => void;
}) {
  const { users, isPending } = useWorkspaceUsers();
  const options: TPickerOption[] = [
    { value: NONE_KEY, label: "Unassigned", icon: <MutedIcon icon={CircleUserIcon} /> },
    ...users.map(userOption),
  ];
  return (
    <PropertyPicker
      label="Assign to"
      options={options}
      value={[assignee?.id ?? NONE_KEY]}
      isPending={isPending}
      onChange={([id]) => {
        if (id === (assignee?.id ?? NONE_KEY)) return;
        const user = users.find((item) => item.id === id);
        onChange(user ? toUserRef(user) : null);
      }}
      {...control}
      trigger={
        <PickerTrigger
          appearance={appearance}
          className={className}
          label={assignee ? `Assignee: ${userName(assignee)}` : "Assign"}
          icon={assignee ? <UserAvatar user={assignee} /> : <MutedIcon icon={CircleUserIcon} />}
          text={assignee ? userName(assignee) : emptyText(appearance, "Unassigned", "Assignee")}
          muted={!assignee}
        />
      }
    />
  );
}

export function SubscribersPicker({
  subscribers,
  onChange,
  appearance = "row",
  className,
  ...control
}: TPickerControl & {
  subscribers: UserRef[];
  onChange: (added: UserRef[], removed: UserRef[]) => void;
}) {
  const { users, isPending } = useWorkspaceUsers();
  const currentIds = subscribers.map((user) => user.id);
  return (
    <PropertyPicker
      label="Subscribers"
      multiple
      options={users.map(userOption)}
      value={currentIds}
      isPending={isPending}
      onChange={(ids) => {
        const added = users
          .filter((user) => ids.includes(user.id) && !currentIds.includes(user.id))
          .map(toUserRef);
        const removed = subscribers.filter((user) => !ids.includes(user.id));
        onChange(added, removed);
      }}
      {...control}
      trigger={
        <PickerTrigger
          appearance={appearance}
          className={className}
          label="Change subscribers"
          icon={
            subscribers.length > 0 ? (
              <span className="flex items-center -space-x-1">
                {subscribers.slice(0, 3).map((user) => (
                  <UserAvatar key={user.id} user={user} size="xs" />
                ))}
              </span>
            ) : (
              <MutedIcon icon={UsersIcon} />
            )
          }
          text={
            subscribers.length === 0
              ? "No subscribers"
              : `${subscribers.length} subscriber${subscribers.length === 1 ? "" : "s"}`
          }
          muted={subscribers.length === 0}
        />
      }
    />
  );
}

export function toLabelRef(label: IssueLabel): LabelRef {
  return { id: label.id, name: label.name, color: label.color, parent: label.parent?.name ?? null };
}

/** Picking a label of a single-select group drops the group's other labels. */
function applySingleSelectGroups(
  previous: string[],
  next: string[],
  labels: IssueLabel[],
  groups: IssueLabel[],
) {
  const added = next.filter((id) => !previous.includes(id));
  return added.reduce((ids, addedId) => {
    const parentId = labels.find((label) => label.id === addedId)?.parent?.id;
    const group = groups.find((item) => item.id === parentId);
    if (group?.groupType !== "singleSelect") return ids;
    const siblings = labels
      .filter((label) => label.parent?.id === group.id && label.id !== addedId)
      .map((label) => label.id);
    return ids.filter((id) => !siblings.includes(id));
  }, next);
}

export function LabelDot({ color }: { color: string | null | undefined }) {
  return (
    <span
      aria-hidden
      className="bg-muted-foreground size-2 shrink-0 rounded-full"
      style={color ? { backgroundColor: color } : undefined}
    />
  );
}

export function LabelsPicker({
  teamId,
  labels,
  onChange,
  appearance = "row",
  className,
  ...control
}: TPickerControl & {
  teamId: string | undefined;
  labels: LabelRef[];
  onChange: (labels: LabelRef[]) => void;
}) {
  const { labels: available, groups, isPending } = useLabels(teamId);
  const currentIds = labels.map((label) => label.id);
  const options: TPickerOption[] = available.map((label) => ({
    value: label.id,
    label: label.name,
    group: label.parent?.name,
    keywords: label.parent ? [label.parent.name] : undefined,
    icon: <LabelDot color={label.color} />,
  }));
  const text =
    labels.length === 0 ? (
      emptyText(appearance, "Add labels", "Labels")
    ) : appearance === "row" ? (
      <span className="flex min-w-0 flex-wrap items-center gap-1.5">
        {labels.map((label) => (
          <LabelChip key={label.id} name={label.name} color={label.color} />
        ))}
      </span>
    ) : labels.length === 1 ? (
      labels[0].name
    ) : (
      `${labels.length} labels`
    );
  return (
    <PropertyPicker
      label="Change labels"
      multiple
      options={options}
      value={currentIds}
      isPending={isPending}
      emptyText="No labels"
      onChange={(ids) => {
        const resolved = applySingleSelectGroups(currentIds, ids, available, groups);
        onChange(
          resolved
            .map((id) => {
              const label = available.find((item) => item.id === id);
              return label ? toLabelRef(label) : labels.find((item) => item.id === id);
            })
            .filter((label): label is LabelRef => Boolean(label)),
        );
      }}
      {...control}
      trigger={
        <PickerTrigger
          appearance={appearance}
          className={className}
          label="Change labels"
          icon={
            labels.length > 0 && appearance === "row" ? null : appearance === "compact" &&
              labels.length > 0 ? (
              <LabelDot color={labels[0].color} />
            ) : (
              <MutedIcon icon={TagIcon} />
            )
          }
          text={text}
          muted={labels.length === 0}
        />
      }
    />
  );
}

export function ProjectPicker({
  teamId,
  project,
  onChange,
  appearance = "row",
  className,
  ...control
}: TPickerControl & {
  teamId: string | undefined;
  project: ProjectRef | null | undefined;
  onChange: (project: ProjectRef | null) => void;
}) {
  const { projects, isPending } = useProjects(teamId);
  const lookup = useProjectLookup();
  const current = project ? lookup.get(project.id) : undefined;
  const options: TPickerOption[] = [
    { value: NONE_KEY, label: "No project", icon: <ProjectIcon /> },
    ...projects.map((item) => ({
      value: item.id,
      label: item.name,
      icon: <ProjectIcon icon={item.icon} color={item.color} />,
    })),
  ];
  return (
    <PropertyPicker
      label="Move to project"
      options={options}
      value={[project?.id ?? NONE_KEY]}
      isPending={isPending}
      onChange={([id]) => {
        if (id === (project?.id ?? NONE_KEY)) return;
        const next = projects.find((item) => item.id === id);
        onChange(next ? { id: next.id, name: next.name, slugId: next.slugId } : null);
      }}
      {...control}
      trigger={
        <PickerTrigger
          appearance={appearance}
          className={className}
          label={project ? `Project: ${project.name}` : "Add to project"}
          icon={<ProjectIcon icon={current?.icon} color={current?.color} />}
          text={project?.name ?? emptyText(appearance, "No project", "Project")}
          muted={!project}
        />
      }
    />
  );
}

export function MilestonePicker({
  projectId,
  milestone,
  onChange,
  appearance = "row",
  className,
  ...control
}: TPickerControl & {
  projectId: string | undefined;
  milestone: MilestoneRef | null | undefined;
  onChange: (milestone: MilestoneRef | null) => void;
}) {
  const { milestones, isPending } = useMilestones(projectId);
  const options: TPickerOption[] = [
    { value: NONE_KEY, label: "No milestone", icon: <MutedIcon icon={DiamondIcon} /> },
    ...milestones.map((item) => ({
      value: item.id,
      label: item.name,
      description: item.targetDate ? formatDate(parseISO(item.targetDate)) : undefined,
      icon: <DiamondIcon className="size-4" />,
    })),
  ];
  return (
    <PropertyPicker
      label="Set milestone"
      options={options}
      value={[milestone?.id ?? NONE_KEY]}
      isPending={isPending}
      emptyText="This project has no milestones"
      onChange={([id]) => {
        if (id === (milestone?.id ?? NONE_KEY)) return;
        const next = milestones.find((item) => item.id === id);
        onChange(next ? { id: next.id, name: next.name, targetDate: next.targetDate } : null);
      }}
      {...control}
      disabled={!projectId || control.disabled}
      trigger={
        <PickerTrigger
          appearance={appearance}
          className={className}
          label={milestone ? `Milestone: ${milestone.name}` : "Set milestone"}
          icon={<MutedIcon icon={DiamondIcon} />}
          text={milestone?.name ?? emptyText(appearance, "No milestone", "Milestone")}
          muted={!milestone}
        />
      }
    />
  );
}

export function CyclePicker({
  teamId,
  cycle,
  onChange,
  appearance = "row",
  className,
  ...control
}: TPickerControl & {
  teamId: string | undefined;
  cycle: CycleRef | null | undefined;
  onChange: (cycle: CycleRef | null) => void;
}) {
  const { cycles, isPending, enabled } = useCycles(teamId);
  if (!enabled) return null;
  const options: TPickerOption[] = [
    { value: NONE_KEY, label: "No cycle", icon: <CycleIcon /> },
    ...cycles.map((item) => ({
      value: item.id,
      label: cycleName(item),
      description: item.isActive ? "Current" : formatDate(item.startsAt),
      icon: <CycleIcon status={item.isActive ? "active" : "upcoming"} />,
    })),
  ];
  return (
    <PropertyPicker
      label="Move to cycle"
      options={options}
      value={[cycle?.id ?? NONE_KEY]}
      isPending={isPending}
      onChange={([id]) => {
        if (id === (cycle?.id ?? NONE_KEY)) return;
        const next = cycles.find((item) => item.id === id);
        onChange(
          next
            ? {
                id: next.id,
                name: next.name,
                number: next.number,
                startsAt: next.startsAt,
                endsAt: next.endsAt,
              }
            : null,
        );
      }}
      {...control}
      trigger={
        <PickerTrigger
          appearance={appearance}
          className={className}
          label={cycle ? `Cycle: ${cycleName(cycle)}` : "Add to cycle"}
          icon={<CycleIcon status={cycle ? "active" : "upcoming"} />}
          text={cycle ? cycleName(cycle) : emptyText(appearance, "No cycle", "Cycle")}
          muted={!cycle}
        />
      }
    />
  );
}

export function EstimatePicker({
  teamId,
  estimate,
  onChange,
  appearance = "row",
  className,
  ...control
}: TPickerControl & {
  teamId: string | undefined;
  estimate: number | null;
  onChange: (estimate: number | null) => void;
}) {
  const team = useTeamDetail(teamId);
  if (!team || team.estimateType === "notUsed") return null;
  const type = team.estimateType;
  const unit = (value: number) => (type === "tShirt" ? "" : value === 1 ? " point" : " points");
  const options: TPickerOption[] = [
    { value: NONE_KEY, label: "No estimate", icon: <MutedIcon icon={TriangleIcon} /> },
    ...team.allowedEstimates.map((value) => ({
      value: String(value),
      label: `${formatEstimate(value, type)}${unit(value)}`,
      icon: <TriangleIcon className="size-4" />,
    })),
  ];
  const text =
    estimate === null
      ? emptyText(appearance, "No estimate", "Estimate")
      : `${formatEstimate(estimate, type)}${unit(estimate)}`;
  return (
    <PropertyPicker
      label="Set estimate"
      options={options}
      value={[estimate === null ? NONE_KEY : String(estimate)]}
      onChange={([value]) => onChange(value === NONE_KEY ? null : Number(value))}
      {...control}
      trigger={
        <PickerTrigger
          appearance={appearance}
          className={className}
          label={estimate === null ? "Set estimate" : `Estimate: ${text}`}
          icon={<MutedIcon icon={TriangleIcon} />}
          text={text}
          muted={estimate === null}
        />
      }
    />
  );
}

export function formatDueDate(value: string) {
  return formatDate(parseISO(value));
}

export function DueDatePicker({
  dueDate,
  onChange,
  appearance = "row",
  className,
  ...control
}: TPickerControl & { dueDate: string | null; onChange: (dueDate: string | null) => void }) {
  return (
    <DatePicker
      name="Due date"
      emptyLabel={emptyText(appearance, "No due date", "Due date")}
      date={dueDate}
      format={formatDueDate}
      onChange={onChange}
      appearance={appearance}
      className={className}
      {...control}
    />
  );
}

export function DatePicker({
  name,
  emptyLabel,
  date,
  format,
  onChange,
  appearance = "row",
  open,
  onOpenChange,
  disabled,
  className,
}: TPickerControl & {
  name: string;
  emptyLabel?: string;
  date: string | null;
  format: (date: string) => string;
  onChange: (date: string | null) => void;
}) {
  const [internalOpen, setInternalOpen] = useState(false);
  const isOpen = open ?? internalOpen;
  const setOpen = (next: boolean) => {
    setInternalOpen(next);
    onOpenChange?.(next);
  };
  const text = date ? format(date) : (emptyLabel ?? name);
  return (
    <span className="contents" onClick={(event) => event.stopPropagation()}>
      <Popover open={isOpen} onOpenChange={setOpen}>
        <PopoverTrigger
          disabled={disabled}
          render={
            <PickerTrigger
              appearance={appearance}
              className={className}
              label={date ? `${name}: ${text}` : `Set ${name.toLowerCase()}`}
              icon={<MutedIcon icon={CalendarIcon} />}
              text={text}
              muted={!date}
            />
          }
        />
        <PopoverContent align="start" className="flex w-64 flex-col gap-2 p-3">
          <BlockPageHotkeys />
          <Input
            type="date"
            aria-label={name}
            autoFocus
            defaultValue={date?.slice(0, 10) ?? ""}
            onChange={(event) => {
              if (!event.target.value) return;
              onChange(event.target.value);
            }}
            onKeyDown={(event) => {
              if (event.key === "Enter") setOpen(false);
            }}
          />
          {date && (
            <Button
              variant="ghost"
              size="sm"
              className="self-start"
              onClick={() => {
                onChange(null);
                setOpen(false);
              }}
            >
              <XIcon className="size-4" />
              Remove {name.toLowerCase()}
            </Button>
          )}
        </PopoverContent>
      </Popover>
    </span>
  );
}

export const toIssueRef = (
  issue: Pick<Issue, "id" | "identifier" | "title" | "state">,
): IssueRef => ({
  id: issue.id,
  identifier: issue.identifier,
  title: issue.title,
  state: issue.state,
});

/** Finds issues by words or identifier and picks one. */
export function IssueSearchPicker({
  label,
  exclude = [],
  onSelect,
  trigger,
  open,
  onOpenChange,
  disabled,
  extraOptions = [],
  onOption,
  header,
}: {
  /** Picked one of extraOptions (like "Create a new sub-issue"); NONE_KEY otherwise selects null. */
  onOption?: (value: string) => void;
  header?: ReactNode;
  label: string;
  exclude?: string[];
  onSelect: (issue: IssueRef | null) => void;
  trigger: ReactElement;
  open?: boolean;
  onOpenChange?: (open: boolean) => void;
  disabled?: boolean;
  extraOptions?: TPickerOption[];
}) {
  const { workspace } = useWorkspace();
  const [query, setQuery] = useState("");
  const [debounced] = useDebounceValue(query, 200);
  const { data, isFetching } = useQuery(searchIssuesQuery(workspace.id, debounced));
  const results = (data ?? []).filter((issue) => !exclude.includes(issue.id));
  const options: TPickerOption[] = [
    ...extraOptions,
    ...results.map((issue) => ({
      value: issue.id,
      label: issue.title,
      description: issue.identifier,
      icon: <StateIcon type={issue.state.type} color={issue.state.color} name={issue.state.name} />,
    })),
  ];
  return (
    <PropertyPicker
      label={label}
      searchPlaceholder="Search issues by title or ID…"
      options={options}
      value={[]}
      onSearchChange={setQuery}
      isPending={isFetching}
      emptyText={debounced ? "No matching issues" : "Type to search issues"}
      open={open}
      onOpenChange={onOpenChange}
      disabled={disabled}
      header={header}
      wide
      onChange={([id]) => {
        if (onOption && extraOptions.some((option) => option.value === id)) {
          onOption(id);
          return;
        }
        if (id === NONE_KEY) {
          onSelect(null);
          return;
        }
        const next = results.find((issue) => issue.id === id);
        if (next) onSelect(toIssueRef(next));
      }}
      trigger={trigger}
    />
  );
}

export function ParentPicker({
  issueId,
  parent,
  onChange,
  appearance = "row",
  className,
  ...control
}: TPickerControl & {
  issueId?: string;
  parent: IssueRef | null | undefined;
  onChange: (parent: IssueRef | null) => void;
}) {
  return (
    <IssueSearchPicker
      label="Set parent issue"
      exclude={issueId ? [issueId] : []}
      extraOptions={
        parent
          ? [{ value: NONE_KEY, label: "Remove parent", icon: <MutedIcon icon={XIcon} /> }]
          : []
      }
      onSelect={onChange}
      open={control.open}
      onOpenChange={control.onOpenChange}
      disabled={control.disabled}
      trigger={
        <PickerTrigger
          appearance={appearance}
          className={className}
          label={parent ? `Parent: ${parent.identifier}` : "Set parent issue"}
          icon={<MutedIcon icon={CornerLeftUpIcon} />}
          text={
            parent ? (
              <>
                <span className="text-muted-foreground font-mono text-xs">{parent.identifier}</span>{" "}
                {parent.title}
              </>
            ) : (
              emptyText(appearance, "No parent", "Parent")
            )
          }
          muted={!parent}
        />
      }
    />
  );
}
