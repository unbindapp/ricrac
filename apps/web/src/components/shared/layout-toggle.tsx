import type { LucideIcon } from "@/components/icons";
import { Button } from "@/components/ui/button";
import { SimpleTooltip } from "@/components/ui/tooltip";

type TLayoutOption<T extends string> = { value: T; label: string; icon: LucideIcon };

type TProps<T extends string> = {
  value: T;
  options: TLayoutOption<T>[];
  onChange: (value: T) => void;
};

/** Icon buttons that switch how a page lays out its things, like a list or a grid. */
export function LayoutToggle<T extends string>({ value, options, onChange }: TProps<T>) {
  return (
    <div
      role="radiogroup"
      aria-label="Layout"
      data-slot="connected-button-group"
      className="flex items-center gap-1"
    >
      {options.map((option) => (
        <SimpleTooltip key={option.value} content={option.label}>
          <Button
            variant={value === option.value ? "secondary" : "ghost"}
            size="icon"
            role="radio"
            aria-checked={value === option.value}
            aria-label={`${option.label} layout`}
            onClick={() => onChange(option.value)}
          >
            <option.icon className="size-4" />
          </Button>
        </SimpleTooltip>
      ))}
    </div>
  );
}
