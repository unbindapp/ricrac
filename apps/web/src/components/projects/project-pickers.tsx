import { useQuery } from "@tanstack/react-query";
import { type ReactElement, useState } from "react";

import type { InitiativeRef, ProjectLabelRef, ProjectStatusRef } from "@/api/ops.gen";
import {
  CalendarIcon,
  ChevronLeftIcon,
  ChevronRightIcon,
  CircleDashedIcon,
  TagIcon,
  TargetIcon,
  XIcon,
} from "@/components/icons";
import { PickerTrigger, type TPickerAppearance } from "@/components/projects/picker-trigger";
import { PropertyPicker, type TPickerOption } from "@/components/issues/property-picker";
import {
  formatProjectDate,
  RESOLUTIONS,
  resolutionLabels,
  snapToResolution,
  type TResolution,
} from "@/components/projects/project-dates";
import { ProjectStatusIcon } from "@/components/projects/project-status-icon";
import { EntityIcon } from "@/components/shared/entity-icon";
import { FilterChip } from "@/components/shared/filter-chip";
import { LabelChip } from "@/components/shared/label-chip";
import { PRIORITIES, PriorityIcon, priorityLabel } from "@/components/shared/priority-icon";
import { ProjectIcon } from "@/components/shared/project-icon";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { initiativesQuery } from "@/lib/queries/initiatives";
import { projectLabelsQuery, projectsQuery, projectStatusesQuery } from "@/lib/queries/projects";

type TControl = { appearance?: TPickerAppearance; disabled?: boolean };

export function useProjectStatuses() {
  const { workspace } = useWorkspace();
  const { data, isPending } = useQuery(projectStatusesQuery(workspace.id));
  return { statuses: data ?? [], isPending };
}

export function ProjectStatusPicker({
  status,
  onChange,
  appearance = "row",
  disabled,
}: TControl & { status: ProjectStatusRef | undefined; onChange: (statusId: string) => void }) {
  const { statuses, isPending } = useProjectStatuses();
  const options: TPickerOption[] = statuses
    .filter((option) => !option.archivedAt)
    .map((option) => ({
      value: option.id,
      label: option.name,
      icon: <ProjectStatusIcon type={option.type} color={option.color} />,
    }));
  return (
    <PropertyPicker
      label="Change status"
      options={options}
      value={status ? [status.id] : []}
      isPending={isPending}
      disabled={disabled}
      onChange={([id]) => {
        if (id && id !== status?.id) onChange(id);
      }}
      trigger={
        <PickerTrigger
          appearance={appearance}
          label={status ? `Status: ${status.name}` : "Set status"}
          icon={
            status ? (
              <ProjectStatusIcon type={status.type} color={status.color} />
            ) : (
              <CircleDashedIcon className="text-muted-foreground size-4" />
            )
          }
          text={status?.name ?? "Status"}
          muted={!status}
        />
      }
    />
  );
}

export function ProjectPriorityPicker({
  priority,
  onChange,
  appearance = "row",
  disabled,
}: TControl & { priority: number; onChange: (priority: number) => void }) {
  const options: TPickerOption[] = [0, 1, 2, 3, 4].map((value) => ({
    value: String(value),
    label: priorityLabel(value),
    icon: <PriorityIcon priority={value} />,
    shortcut: String(value),
  }));
  const isSet = priority !== PRIORITIES[0];
  return (
    <PropertyPicker
      label="Change priority"
      options={options}
      value={[String(priority)]}
      disabled={disabled}
      onChange={([value]) => {
        if (Number(value) !== priority) onChange(Number(value));
      }}
      trigger={
        <PickerTrigger
          appearance={appearance}
          label={`Priority: ${priorityLabel(priority)}`}
          icon={<PriorityIcon priority={priority} />}
          text={isSet || appearance === "row" ? priorityLabel(priority) : "Priority"}
          muted={!isSet}
        />
      }
    />
  );
}

export function ProjectLabelsPicker({
  labels,
  onChange,
  appearance = "row",
  disabled,
}: TControl & { labels: ProjectLabelRef[]; onChange: (ids: string[]) => void }) {
  const { workspace } = useWorkspace();
  const { data, isPending } = useQuery(projectLabelsQuery(workspace.id));
  const options: TPickerOption[] = (data ?? [])
    .filter((label) => !label.isGroup && !label.retiredAt)
    .map((label) => ({
      value: label.id,
      label: label.name,
      group: label.parent?.name,
      icon: (
        <span
          aria-hidden
          className="size-2 shrink-0 rounded-full"
          style={{ backgroundColor: label.color }}
        />
      ),
    }));
  const trigger =
    appearance === "row" && labels.length > 0 ? (
      <Button
        variant="ghost"
        size="sm"
        forceMinSize={false}
        aria-label="Change labels"
        className="dank:text-m3-on-surface h-auto min-h-8 w-full flex-wrap justify-start gap-1 py-1"
      >
        {labels.map((label) => (
          <LabelChip key={label.id} name={label.name} color={label.color} />
        ))}
      </Button>
    ) : (
      <PickerTrigger
        appearance={appearance}
        label="Change labels"
        icon={<TagIcon className="text-muted-foreground size-4" />}
        text={
          labels.length === 0
            ? appearance === "row"
              ? "No labels"
              : "Labels"
            : `${labels.length} labels`
        }
        muted={labels.length === 0}
      />
    );
  return (
    <PropertyPicker
      label="Change labels"
      multiple
      options={options}
      value={labels.map((label) => label.id)}
      isPending={isPending}
      disabled={disabled}
      emptyText="No project labels yet"
      onChange={onChange}
      trigger={trigger}
    />
  );
}

/** Picks projects of the workspace: dependencies, an initiative's projects. */
export function ProjectsPicker({
  label,
  value,
  exclude = [],
  onChange,
  trigger,
}: {
  label: string;
  value: string[];
  exclude?: string[];
  onChange: (ids: string[]) => void;
  trigger: ReactElement;
}) {
  const { workspace } = useWorkspace();
  const { data, isPending } = useQuery(projectsQuery(workspace.id));
  const options: TPickerOption[] = (data ?? [])
    .filter((project) => !exclude.includes(project.id))
    .map((project) => ({
      value: project.id,
      label: project.name,
      icon: <ProjectIcon icon={project.icon} color={project.color} />,
      description: project.status.name,
    }));
  return (
    <PropertyPicker
      label={label}
      multiple
      align="end"
      options={options}
      value={value}
      isPending={isPending}
      emptyText="No projects"
      onChange={onChange}
      trigger={trigger}
    />
  );
}

export function InitiativesPicker({
  initiatives,
  onChange,
  appearance = "row",
  disabled,
}: TControl & { initiatives: InitiativeRef[]; onChange: (ids: string[]) => void }) {
  const { workspace } = useWorkspace();
  const { data, isPending } = useQuery({ ...initiativesQuery(workspace.id), enabled: !disabled });
  const options: TPickerOption[] = (data ?? []).map((initiative) => ({
    value: initiative.id,
    label: initiative.name,
    icon: <EntityIcon icon={initiative.icon} color={initiative.color} fallback={TargetIcon} />,
  }));
  const count = initiatives.length;
  return (
    <PropertyPicker
      label="Change initiatives"
      multiple
      options={options}
      value={initiatives.map((initiative) => initiative.id)}
      isPending={isPending}
      disabled={disabled}
      emptyText="No initiatives yet"
      onChange={onChange}
      trigger={
        <PickerTrigger
          appearance={appearance}
          label="Change initiatives"
          icon={<TargetIcon className="text-muted-foreground size-4" />}
          text={
            count === 0
              ? appearance === "row"
                ? "No initiatives"
                : "Initiatives"
              : initiatives.map((initiative) => initiative.name).join(", ")
          }
          muted={count === 0}
        />
      }
    />
  );
}

const MONTH_NAMES = [
  "Jan",
  "Feb",
  "Mar",
  "Apr",
  "May",
  "Jun",
  "Jul",
  "Aug",
  "Sep",
  "Oct",
  "Nov",
  "Dec",
];

/** The periods of a year to pick from (months, quarters, halves), or a run of years. */
function periodsOf(resolution: Exclude<TResolution, "day">, year: number) {
  const month = (index: number) => `${year}-${String(index + 1).padStart(2, "0")}-01`;
  switch (resolution) {
    case "month":
      return MONTH_NAMES.map((name, index) => ({ label: name, date: month(index) }));
    case "quarter":
      return [1, 2, 3, 4].map((quarter) => ({
        label: `Q${quarter}`,
        date: month((quarter - 1) * 3),
      }));
    case "halfYear":
      return [1, 2].map((half) => ({ label: `H${half}`, date: month((half - 1) * 6) }));
    case "year":
      return [0, 1, 2, 3, 4, 5].map((offset) => ({
        label: String(year + offset),
        date: `${year + offset}-01-01`,
      }));
  }
}

function PeriodGrid({
  resolution,
  date,
  onPick,
}: {
  resolution: Exclude<TResolution, "day">;
  date: string | null;
  onPick: (date: string) => void;
}) {
  const thisYear = new Date().getFullYear();
  const [year, setYear] = useState(() =>
    date ? Number(date.slice(0, 4)) - (resolution === "year" ? 1 : 0) : thisYear,
  );
  const step = resolution === "year" ? 6 : 1;
  const current = date ? formatProjectDate(date, resolution) : null;
  return (
    <div className="flex flex-col gap-2">
      <div className="flex items-center gap-2">
        <Button
          variant="ghost"
          size="icon"
          aria-label="Earlier"
          onClick={() => setYear((value) => value - step)}
        >
          <ChevronLeftIcon className="size-4" />
        </Button>
        <span className="flex-1 text-center text-sm font-medium tabular-nums">
          {resolution === "year" ? `${year} – ${year + 5}` : year}
        </span>
        <Button
          variant="ghost"
          size="icon"
          aria-label="Later"
          onClick={() => setYear((value) => value + step)}
        >
          <ChevronRightIcon className="size-4" />
        </Button>
      </div>
      <div
        data-columns={resolution === "halfYear" ? 2 : resolution === "quarter" ? 4 : 3}
        className="grid grid-cols-3 gap-1 data-[columns=2]:grid-cols-2 data-[columns=4]:grid-cols-4"
      >
        {periodsOf(resolution, year).map((period) => {
          const isCurrent = formatProjectDate(period.date, resolution) === current;
          return (
            <Button
              key={period.date}
              variant={isCurrent ? "secondary" : "ghost"}
              size="sm"
              forceMinSize={false}
              aria-pressed={isCurrent}
              className="dank:type-body-large dank:text-m3-on-surface-variant dank:aria-pressed:bg-m3-primary dank:aria-pressed:text-m3-on-primary dank:h-9 dank:rounded-full dank:active:rounded-full"
              onClick={() => onPick(period.date)}
            >
              {period.label}
            </Button>
          );
        })}
      </div>
    </div>
  );
}

const resolutionOptions = RESOLUTIONS.map((value) => ({ value, label: resolutionLabels[value] }));

/** A planned date at a precision: an exact day, or a month, quarter, half or year. */
export function PlannedDatePicker({
  label,
  edge,
  date,
  resolution,
  onChange,
  appearance = "row",
  disabled,
}: TControl & {
  label: string;
  /** Start dates snap to the start of their period, target dates to its end. */
  edge: "start" | "end";
  date: string | null;
  resolution: TResolution;
  onChange: (value: { date: string | null; resolution: TResolution }) => void;
}) {
  const [isOpen, setIsOpen] = useState(false);
  const [draft, setDraft] = useState<TResolution>(resolution);
  const text = date
    ? formatProjectDate(date, resolution)
    : appearance === "row"
      ? `No ${label.toLowerCase()}`
      : label;
  const pick = (value: string, nextResolution: TResolution) =>
    onChange({ date: snapToResolution(value, nextResolution, edge), resolution: nextResolution });
  return (
    <Popover
      open={isOpen}
      onOpenChange={(open) => {
        setIsOpen(open);
        if (open) setDraft(resolution);
      }}
    >
      <PopoverTrigger
        disabled={disabled}
        render={
          <PickerTrigger
            appearance={appearance}
            label={date ? `${label}: ${text}` : `Set ${label.toLowerCase()}`}
            icon={<CalendarIcon className="text-muted-foreground size-4" />}
            text={text}
            muted={!date}
          />
        }
      />
      <PopoverContent align="start" className="flex w-80 flex-col gap-3 p-3">
        <div role="radiogroup" aria-label={`${label} precision`} className="flex flex-wrap gap-1">
          {resolutionOptions.map((option) => (
            <FilterChip
              key={option.value}
              role="radio"
              aria-checked={draft === option.value}
              selected={draft === option.value}
              forceMinSize={false}
              onClick={() => {
                setDraft(option.value);
                if (date) pick(date, option.value);
              }}
            >
              {option.label}
            </FilterChip>
          ))}
        </div>
        {draft === "day" ? (
          <Input
            type="date"
            aria-label={label}
            defaultValue={date ?? ""}
            onChange={(event) => {
              if (event.target.value) pick(event.target.value, "day");
            }}
            onKeyDown={(event) => {
              if (event.key === "Enter") setIsOpen(false);
            }}
          />
        ) : (
          <PeriodGrid
            resolution={draft}
            date={date}
            onPick={(value) => {
              pick(value, draft);
              setIsOpen(false);
            }}
          />
        )}
        {date && resolution !== "day" && (
          <p className="text-muted-foreground text-xs">
            {edge === "end" ? "By" : "From"} {formatProjectDate(date)}
          </p>
        )}
        {date && (
          <Button
            variant="ghost"
            size="sm"
            className="self-start"
            onClick={() => {
              onChange({ date: null, resolution: "day" });
              setIsOpen(false);
            }}
          >
            <XIcon className="size-4" />
            Remove {label.toLowerCase()}
          </Button>
        )}
      </PopoverContent>
    </Popover>
  );
}
