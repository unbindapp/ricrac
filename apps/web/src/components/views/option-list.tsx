import { CheckIcon } from "@/components/icons/symbols.gen";
import {
  Command,
  CommandEmpty,
  CommandInput,
  CommandItem,
  CommandList,
} from "@/components/ui/command";
import type { TFilterOption } from "@/components/views/use-filter-options";

type TProps = {
  options: TFilterOption[];
  values: string[];
  onToggle: (value: string) => void;
  searchPlaceholder: string;
};

/** A searchable checklist, for popovers that pick several things. */
export function OptionList({ options, values, onToggle, searchPlaceholder }: TProps) {
  return (
    <Command wrapper="none">
      {options.length > 8 && <CommandInput placeholder={searchPlaceholder} />}
      <CommandList className="max-h-72 overflow-y-auto">
        <CommandEmpty>Nothing matches.</CommandEmpty>
        {options.map((option) => {
          const checked = values.includes(option.value);
          return (
            <CommandItem
              key={option.value}
              value={`${option.label} ${option.hint ?? ""} ${option.value}`}
              onSelect={() => onToggle(option.value)}
            >
              <span
                data-checked={checked || undefined}
                className="data-checked:bg-primary data-checked:border-primary text-primary-foreground flex size-4 shrink-0 items-center justify-center rounded-sm border"
              >
                {checked && <CheckIcon className="size-3" />}
              </span>
              {option.icon}
              <span className="min-w-0 flex-1 truncate">{option.label}</span>
              {option.hint && (
                <span className="text-muted-foreground truncate text-xs">{option.hint}</span>
              )}
            </CommandItem>
          );
        })}
      </CommandList>
    </Command>
  );
}
