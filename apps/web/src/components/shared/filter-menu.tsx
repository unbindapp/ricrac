import { type ReactNode } from "react";

import { CheckIcon, ChevronDownIcon } from "@/components/icons/symbols.gen";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";

export type TFilterOption<T extends string> = {
  value: T;
  label: string;
  icon?: ReactNode;
  /** Shown after the label, like how many there are. */
  suffix?: ReactNode;
};

type TProps<T extends string> = {
  label: string;
  value: T;
  options: TFilterOption<T>[];
  onChange: (value: T) => void;
};

/** Picks one of a few ways to look at a list, from a menu button. */
export function FilterMenu<T extends string>({ label, value, options, onChange }: TProps<T>) {
  const current = options.find((option) => option.value === value) ?? options[0];
  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        render={
          <Button
            variant="secondary"
            size="sm"
            aria-label={`${label}: ${current.label}`}
            className="w-fit"
          >
            {current.icon}
            <span>{current.label}</span>
            {current.suffix}
            <ChevronDownIcon className="size-4 transition-transform group-data-popup-open/button:rotate-180" />
          </Button>
        }
      />
      <DropdownMenuContent className="min-w-48">
        <DropdownMenuGroup>
          {options.map((option) => (
            <DropdownMenuItem
              key={option.value}
              data-selected={option.value === value || undefined}
              className="data-selected:bg-secondary data-selected:text-secondary-foreground"
              onClick={() => onChange(option.value)}
            >
              {option.icon}
              <span className="min-w-0 flex-1 truncate">{option.label}</span>
              {option.suffix}
              {option.value === value && <CheckIcon className="size-4" />}
            </DropdownMenuItem>
          ))}
        </DropdownMenuGroup>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
