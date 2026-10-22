import { type ReactNode, useState } from "react";

import type { UserRef } from "@/api/ops.gen";
import {
  BoxIcon,
  CalendarIcon,
  CircleDotIcon,
  CircleUserIcon,
  CornerLeftUpIcon,
  DiamondIcon,
  type LucideIcon,
  PlusIcon,
  RefreshCwIcon,
  RulerIcon,
  SignalHighIcon,
  TagIcon,
} from "@/components/icons";
import { applyPatch, type TIssueDraft, type TIssuePatch } from "@/components/issues/issue-draft";
import { useCycles, useTeamDetail } from "@/components/issues/picker-sources";
import {
  AssigneePicker,
  CyclePicker,
  DueDatePicker,
  EstimatePicker,
  LabelsPicker,
  MilestonePicker,
  ParentPicker,
  PriorityPicker,
  ProjectPicker,
  StatePicker,
  SubscribersPicker,
  type TPickerAppearance,
} from "@/components/issues/pickers";
import { PropertyRow } from "@/components/layout/property-row";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { useHotkeyScope } from "@/lib/hotkeys/use-hotkey-scope";

export type TPropertyKey =
  | "state"
  | "priority"
  | "assignee"
  | "labels"
  | "project"
  | "milestone"
  | "cycle"
  | "estimate"
  | "due_date"
  | "parent";

type TSubscribers = {
  users: UserRef[];
  onChange: (added: UserRef[], removed: UserRef[]) => void;
};

type TProps = {
  teamId: string;
  issueId?: string;
  value: TIssueDraft;
  onChange: (patch: TIssuePatch) => void;
  layout?: "column" | "inline";
  shortcutsEnabled?: boolean;
  subscribers?: TSubscribers;
  disabled?: boolean;
  omit?: TPropertyKey[];
  /** Inline, show only the properties that are set, and the others behind one "+" menu. */
  hideEmpty?: boolean;
  /** More rows at the end of the panel (sharing, SLA, snooze). */
  children?: ReactNode;
};

const SHORTCUTS: { hotkey: string; property: TPropertyKey }[] = [
  { hotkey: "S", property: "state" },
  { hotkey: "P", property: "priority" },
  { hotkey: "A", property: "assignee" },
  { hotkey: "L", property: "labels" },
  { hotkey: "Shift+P", property: "project" },
  { hotkey: "Shift+C", property: "cycle" },
  { hotkey: "E", property: "estimate" },
  { hotkey: "D", property: "due_date" },
];

export function IssueProperties({
  teamId,
  issueId,
  value: draft,
  onChange,
  layout = "column",
  shortcutsEnabled = false,
  subscribers,
  disabled,
  omit = [],
  hideEmpty = false,
  children,
}: TProps) {
  const [openProperty, setOpenProperty] = useState<TPropertyKey | null>(null);
  const team = useTeamDetail(teamId);
  const { enabled: cyclesEnabled } = useCycles(teamId);
  const estimatesEnabled = Boolean(team && team.estimateType !== "notUsed");
  useHotkeyScope(
    SHORTCUTS.map(({ hotkey, property }) => ({
      hotkey,
      callback: () => setOpenProperty((current) => current ?? property),
    })),
    { enabled: shortcutsEnabled && !disabled },
  );

  const appearance: TPickerAppearance = layout === "column" ? "row" : "compact";
  const control = (property: TPropertyKey) => ({
    appearance,
    disabled,
    open: openProperty === property,
    onOpenChange: (open: boolean) => setOpenProperty(open ? property : null),
  });

  const fields: {
    property: TPropertyKey;
    label: string;
    icon: LucideIcon;
    node: ReactNode;
    hidden?: boolean;
  }[] = [
    {
      property: "state",
      label: "Status",
      icon: CircleDotIcon,
      node: (
        <StatePicker
          teamId={teamId}
          state={draft.state}
          onChange={(state) => onChange({ state })}
          {...control("state")}
        />
      ),
    },
    {
      property: "priority",
      label: "Priority",
      icon: SignalHighIcon,
      node: (
        <PriorityPicker
          priority={draft.priority}
          onChange={(priority) => onChange({ priority })}
          {...control("priority")}
        />
      ),
    },
    {
      property: "assignee",
      label: "Assignee",
      icon: CircleUserIcon,
      node: (
        <AssigneePicker
          assignee={draft.assignee}
          onChange={(assignee) => onChange({ assignee })}
          {...control("assignee")}
        />
      ),
    },
    {
      property: "labels",
      label: "Labels",
      icon: TagIcon,
      node: (
        <LabelsPicker
          teamId={teamId}
          labels={draft.labels}
          onChange={(labels) => onChange({ labels })}
          {...control("labels")}
        />
      ),
    },
    {
      property: "project",
      label: "Project",
      icon: BoxIcon,
      node: (
        <ProjectPicker
          teamId={teamId}
          project={draft.project}
          onChange={(project) => onChange(withMilestone(draft, { project }))}
          {...control("project")}
        />
      ),
    },
    {
      property: "milestone",
      label: "Milestone",
      icon: DiamondIcon,
      hidden: !draft.project,
      node: (
        <MilestonePicker
          projectId={draft.project?.id}
          milestone={draft.projectMilestone}
          onChange={(projectMilestone) => onChange({ projectMilestone })}
          {...control("milestone")}
        />
      ),
    },
    {
      property: "cycle",
      label: "Cycle",
      icon: RefreshCwIcon,
      hidden: !cyclesEnabled,
      node: (
        <CyclePicker
          teamId={teamId}
          cycle={draft.cycle}
          onChange={(cycle) => onChange({ cycle })}
          {...control("cycle")}
        />
      ),
    },
    {
      property: "estimate",
      label: "Estimate",
      icon: RulerIcon,
      hidden: !estimatesEnabled,
      node: (
        <EstimatePicker
          teamId={teamId}
          estimate={draft.estimate}
          onChange={(estimate) => onChange({ estimate })}
          {...control("estimate")}
        />
      ),
    },
    {
      property: "due_date",
      label: "Due date",
      icon: CalendarIcon,
      node: (
        <DueDatePicker
          dueDate={draft.dueDate}
          onChange={(dueDate) => onChange({ dueDate })}
          {...control("due_date")}
        />
      ),
    },
    {
      property: "parent",
      label: "Parent",
      icon: CornerLeftUpIcon,
      node: (
        <ParentPicker
          issueId={issueId}
          parent={draft.parent}
          onChange={(parent) => onChange({ parent })}
          {...control("parent")}
        />
      ),
    },
  ];
  const visible = fields.filter((field) => !field.hidden && !omit.includes(field.property));

  if (layout === "inline") {
    const isShown = (property: TPropertyKey) =>
      !hideEmpty || openProperty === property || isSet(draft, property);
    const unset = visible.filter((field) => !isShown(field.property));
    return (
      <div className="flex flex-col gap-2">
        <div className="flex flex-wrap items-center gap-2">
          {visible
            .filter((field) => isShown(field.property))
            .map((field) => (
              <span key={field.property} className="contents">
                {field.node}
              </span>
            ))}
          {!disabled && unset.length > 0 && (
            <DropdownMenu>
              <DropdownMenuTrigger
                render={
                  <Button
                    variant="outline"
                    size="sm"
                    forceMinSize={false}
                    aria-label="Add property"
                    className="text-muted-foreground px-2"
                  >
                    <PlusIcon className="size-4" />
                  </Button>
                }
              />
              <DropdownMenuContent align="start" className="w-44">
                {unset.map((field) => (
                  <DropdownMenuItem
                    key={field.property}
                    onClick={() => setOpenProperty(field.property)}
                  >
                    <field.icon className="size-4" />
                    {field.label}
                  </DropdownMenuItem>
                ))}
              </DropdownMenuContent>
            </DropdownMenu>
          )}
        </div>
        {children}
      </div>
    );
  }

  return (
    <div className="flex w-full flex-col gap-0.5">
      {visible.map((field) => (
        <PropertyRow key={field.property} label={field.label}>
          {field.node}
        </PropertyRow>
      ))}
      {subscribers && (
        <PropertyRow label="Subscribers">
          <SubscribersPicker
            subscribers={subscribers.users}
            onChange={subscribers.onChange}
            appearance="row"
            disabled={disabled}
          />
        </PropertyRow>
      )}
      {children}
    </div>
  );
}

function isSet(draft: TIssueDraft, property: TPropertyKey) {
  switch (property) {
    case "state":
      return true;
    case "priority":
      return draft.priority !== 0;
    case "assignee":
      return Boolean(draft.assignee);
    case "labels":
      return draft.labels.length > 0;
    case "project":
      return Boolean(draft.project);
    case "milestone":
      return Boolean(draft.projectMilestone);
    case "cycle":
      return Boolean(draft.cycle);
    case "estimate":
      return draft.estimate !== null;
    case "due_date":
      return Boolean(draft.dueDate);
    case "parent":
      return Boolean(draft.parent);
  }
}

/** Changing the project drops a milestone of the old one. */
function withMilestone(draft: TIssueDraft, patch: TIssuePatch): TIssuePatch {
  const next = applyPatch(draft, patch);
  if (next.projectMilestone === draft.projectMilestone) return patch;
  return { ...patch, projectMilestone: next.projectMilestone };
}
