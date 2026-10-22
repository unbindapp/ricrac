import { CheckIcon } from "@/components/icons/symbols.gen";
import { cn } from "@/components/ui/utils";

type TProps = {
  checked: boolean;
  hasChanges?: boolean;
  className?: string;
  size?: "sm" | "md";
};

export function ToggleKnob({ checked, hasChanges, size = "md", className }: TProps) {
  return (
    <div
      data-checked={checked || undefined}
      data-staged={hasChanges || undefined}
      data-size={size}
      className={cn(
        // The color is animated through currentColor: Chrome runs background-color transitions
        // on the compositor and can paint the start color for a frame when they end
        "group/knob text-surface-container-highest border-muted-more-foreground data-checked:text-primary data-checked:border-primary data-staged:data-checked:text-change data-staged:data-checked:border-change relative h-8 w-13 shrink-0 rounded-full border-2 bg-current transition data-[size=sm]:h-6 data-[size=sm]:w-10",
        className,
      )}
    >
      <div className="bg-muted-more-foreground group-data-checked/knob:bg-primary-foreground group-data-checked/knob:text-primary group-data-staged/knob:group-data-checked/knob:text-change ease-emphasized absolute top-1/2 left-1.5 flex size-4 -translate-y-1/2 items-center justify-center rounded-full transition-all duration-300 group-active/switch:left-0 group-active/switch:size-7 group-data-checked/knob:left-5.5 group-data-checked/knob:size-6 group-data-checked/knob:group-active/switch:left-5 group-data-checked/knob:group-active/switch:size-7 group-data-[size=sm]/knob:left-1 group-data-[size=sm]/knob:size-3 group-data-[size=sm]/knob:group-active/switch:left-0 group-data-[size=sm]/knob:group-active/switch:size-5 group-data-checked/knob:group-data-[size=sm]/knob:left-4.25 group-data-checked/knob:group-data-[size=sm]/knob:size-4.5 group-data-checked/knob:group-data-[size=sm]/knob:group-active/switch:left-4 group-data-checked/knob:group-data-[size=sm]/knob:group-active/switch:size-5">
        <CheckIcon className="size-3.5 opacity-0 transition-opacity group-data-checked/knob:opacity-100 group-data-[size=sm]/knob:size-3" />
      </div>
    </div>
  );
}
