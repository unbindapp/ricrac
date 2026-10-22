import { type ReactNode, useId } from "react";
import { ChevronsUpDownIcon } from "lucide-react";

import { Button } from "@/components/ui/button";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { OptionList } from "@/components/views/option-list";
import type { TFilterOption } from "@/components/views/use-filter-options";

export function Field({
  label,
  hint,
  children,
}: {
  label: string;
  hint?: ReactNode;
  children: (id: string) => ReactNode;
}) {
  const id = useId();
  return (
    <div className="flex min-w-0 flex-col gap-1.5">
      <label htmlFor={id} className="text-sm font-medium">
        {label}
      </label>
      {children(id)}
      {hint && <p className="text-muted-foreground text-xs">{hint}</p>}
    </div>
  );
}

export function SimpleSelect({
  id,
  label,
  items,
  value,
  onChange,
  size,
}: {
  id?: string;
  label: string;
  items: TFilterOption[];
  value: string;
  onChange: (value: string) => void;
  size?: "sm" | "default";
}) {
  const selected = items.find((item) => item.value === value);
  return (
    <Select items={items} value={value} onValueChange={(next) => onChange(next ?? "")}>
      <SelectTrigger id={id} aria-label={label} size={size}>
        <SelectValue>
          {selected && (
            <>
              {selected.icon}
              <span className="truncate">{selected.label}</span>
            </>
          )}
        </SelectValue>
      </SelectTrigger>
      <SelectContent>
        {items.map((item) => (
          <SelectItem key={item.value} value={item.value}>
            {item.icon}
            <span className="truncate">{item.label}</span>
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  );
}

/** A button that shows what's picked and opens a searchable checklist. */
export function MultiPicker({
  id,
  label,
  options,
  values,
  onChange,
  placeholder,
}: {
  id?: string;
  label: string;
  options: TFilterOption[];
  values: string[];
  onChange: (values: string[]) => void;
  placeholder: string;
}) {
  const picked = values.map(
    (value) => options.find((option) => option.value === value) ?? { value, label: value },
  );
  const toggle = (value: string) =>
    onChange(values.includes(value) ? values.filter((item) => item !== value) : [...values, value]);
  return (
    <Popover>
      <PopoverTrigger
        render={
          <Button
            id={id}
            variant="outline"
            aria-label={label}
            className="h-auto min-h-9 w-full justify-start px-3 py-1.5 font-normal"
          >
            <span className="flex min-w-0 flex-1 flex-wrap items-center gap-1.5">
              {picked.length === 0 && <span className="text-muted-foreground">{placeholder}</span>}
              {picked.map((option) => (
                <span
                  key={option.value}
                  className="bg-muted flex h-6 max-w-48 items-center gap-1.5 rounded-md px-2 text-xs"
                >
                  {option.icon}
                  <span className="truncate">{option.label}</span>
                </span>
              ))}
            </span>
            <ChevronsUpDownIcon className="text-muted-foreground size-3.5" />
          </Button>
        }
      />
      <PopoverContent align="start" className="w-72 p-0">
        <OptionList
          options={options}
          values={values}
          onToggle={toggle}
          searchPlaceholder={`Find ${label.toLowerCase()}`}
        />
      </PopoverContent>
    </Popover>
  );
}
