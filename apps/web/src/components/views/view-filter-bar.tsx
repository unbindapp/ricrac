import { useState } from "react";

import { ListFilterIcon, XIcon } from "@/components/icons/symbols.gen";
import {
  FIELDS,
  IS_ONLY,
  type TCondition,
  type TFilterField,
  type TFilterModel,
} from "@/components/views/view-filter";
import {
  fieldLabels,
  type TFilterOption,
  useFilterOptions,
} from "@/components/views/use-filter-options";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { OptionList } from "@/components/views/option-list";

type TProps = {
  model: TFilterModel;
  conditions: TCondition[];
  onChange: (conditions: TCondition[]) => void;
  disabled?: boolean;
};

/** Filter chips (field, is / is not, values) and a menu to add one. */
export function ViewFilterBar({ model, conditions, onChange, disabled }: TProps) {
  const options = useFilterOptions();
  const [justAdded, setJustAdded] = useState<TFilterField | null>(null);
  const unused = FIELDS[model].filter(
    (field) => !conditions.some((condition) => condition.field === field),
  );
  const update = (index: number, next: TCondition | null) =>
    onChange(
      next
        ? conditions.map((condition, n) => (n === index ? next : condition))
        : conditions.filter((_, n) => n !== index),
    );

  return (
    <div className="flex min-w-0 flex-wrap items-center gap-2">
      {conditions.map((condition, index) => (
        <ConditionChip
          key={condition.field}
          condition={condition}
          options={options[condition.field]}
          autoOpen={justAdded === condition.field}
          disabled={disabled}
          onChange={(next) => update(index, next)}
        />
      ))}
      {!disabled && unused.length > 0 && (
        <DropdownMenu>
          <DropdownMenuTrigger
            render={
              <Button variant="ghost" size="sm" className="text-muted-foreground">
                <ListFilterIcon className="size-4" />
                {conditions.length === 0 ? "Filter" : "Add filter"}
              </Button>
            }
          />
          <DropdownMenuContent align="start" className="w-44">
            {unused.map((field) => (
              <DropdownMenuItem
                key={field}
                onClick={() => {
                  setJustAdded(field);
                  onChange([...conditions, { field, op: "is", values: [] }]);
                }}
              >
                {fieldLabels[field]}
              </DropdownMenuItem>
            ))}
          </DropdownMenuContent>
        </DropdownMenu>
      )}
    </div>
  );
}

function opLabel(condition: TCondition) {
  const many = condition.values.length > 1;
  if (condition.op === "is") return many ? "is any of" : "is";
  return many ? "is none of" : "is not";
}

function valuesLabel(condition: TCondition, options: TFilterOption[]) {
  if (condition.values.length === 0) return { text: "Pick…", icon: null };
  const picked = options.filter((option) => condition.values.includes(option.value));
  if (condition.values.length === 1) {
    return { text: picked[0]?.label ?? condition.values[0], icon: picked[0]?.icon ?? null };
  }
  const names = picked.map((option) => option.label).join(", ");
  if (names.length <= 28) return { text: names, icon: null };
  return { text: `${condition.values.length} ${pluralLabels[condition.field]}`, icon: null };
}

const pluralLabels: Record<TFilterField, string> = {
  statusType: "status types",
  status: "statuses",
  assignee: "people",
  creator: "people",
  priority: "priorities",
  labels: "labels",
  project: "projects",
  cycle: "cycles",
  dueDate: "dates",
  projectStatus: "statuses",
  lead: "people",
  projectPriority: "priorities",
  health: "health values",
};

function ConditionChip({
  condition,
  options,
  autoOpen,
  disabled,
  onChange,
}: {
  condition: TCondition;
  options: TFilterOption[];
  autoOpen: boolean;
  disabled?: boolean;
  onChange: (condition: TCondition | null) => void;
}) {
  const [isOpen, setIsOpen] = useState(autoOpen);
  const label = valuesLabel(condition, options);
  const canNegate = !IS_ONLY.includes(condition.field);
  const toggle = (value: string) => {
    const values = condition.values.includes(value)
      ? condition.values.filter((item) => item !== value)
      : [...condition.values, value];
    onChange({ ...condition, values });
  };

  return (
    <div className="bg-background flex h-7 items-center overflow-hidden rounded-md border text-xs">
      <span className="text-muted-foreground flex h-full items-center px-2">
        {fieldLabels[condition.field]}
      </span>
      {canNegate ? (
        <DropdownMenu>
          <DropdownMenuTrigger
            disabled={disabled}
            render={
              <Button
                variant="ghost"
                size="sm"
                forceMinSize={false}
                className="h-full rounded-none border-l px-2 text-xs font-normal"
              >
                {opLabel(condition)}
              </Button>
            }
          />
          <DropdownMenuContent align="start" className="w-32">
            <DropdownMenuItem onClick={() => onChange({ ...condition, op: "is" })}>
              is
            </DropdownMenuItem>
            <DropdownMenuItem onClick={() => onChange({ ...condition, op: "isNot" })}>
              is not
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      ) : (
        <span className="text-muted-foreground flex h-full items-center border-l px-2">is</span>
      )}
      <Popover
        open={isOpen}
        onOpenChange={(open) => {
          setIsOpen(open);
          if (!open && condition.values.length === 0) onChange(null);
        }}
      >
        <PopoverTrigger
          disabled={disabled}
          render={
            <Button
              variant="ghost"
              size="sm"
              forceMinSize={false}
              data-empty={condition.values.length === 0 || undefined}
              className="data-empty:text-muted-foreground h-full max-w-56 rounded-none border-l px-2 text-xs font-medium"
            >
              {label.icon}
              <span className="truncate">{label.text}</span>
            </Button>
          }
        />
        <PopoverContent align="start" className="w-64 p-0">
          <OptionList
            options={options}
            values={condition.values}
            onToggle={toggle}
            searchPlaceholder={`Filter ${fieldLabels[condition.field].toLowerCase()}`}
          />
        </PopoverContent>
      </Popover>
      {!disabled && (
        <Button
          variant="ghost"
          size="icon"
          forceMinSize={false}
          aria-label={`Remove the ${fieldLabels[condition.field].toLowerCase()} filter`}
          onClick={() => onChange(null)}
          className="text-muted-foreground size-7 rounded-none border-l"
        >
          <XIcon className="size-3.5" />
        </Button>
      )}
    </div>
  );
}
